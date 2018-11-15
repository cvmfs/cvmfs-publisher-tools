package consume

import (
	"encoding/json"
	"os"

	"github.com/cvmfs/cvmfs-publisher-tools/internal/job"
	"github.com/cvmfs/cvmfs-publisher-tools/internal/log"
	"github.com/cvmfs/cvmfs-publisher-tools/internal/queue"
	"github.com/cvmfs/cvmfs-publisher-tools/internal/transaction"
	getter "github.com/hashicorp/go-getter"
	"github.com/streadway/amqp"
)

// Run - runs the job consumer
func Run(qcfg queue.Config, tempDir string) {
	// Create temporary dir
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Error.Println("Could not create temp dir:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	conn, err := queue.NewConnection(qcfg)
	if err != nil {
		log.Error.Println("Could not create job queue connection:", err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := conn.SetupTopology(); err != nil {
		log.Error.Println("Could not set up RabbitMQ topology:", err)
		os.Exit(1)
	}

	jobs, err := conn.Chan.Consume(
		conn.Queue.Name, queue.ConsumerName, false, false, false, false, nil)
	if err != nil {
		log.Error.Println("Could not start consuming jobs:", err)
		os.Exit(1)
	}

	go func() {
		ch := conn.Chan.NotifyClose(make(chan *amqp.Error))
		err := <-ch
		log.Error.Println("Connection to job queue closed:", err)
		os.Exit(1)
	}()

	log.Info.Println("Waiting for jobs")

	var desc job.Description
	for j := range jobs {
		if err := json.Unmarshal(j.Body, &desc); err != nil {
			log.Error.Println("Could not unmarshal job description:", err)
			j.Nack(false, false)
			continue
		}
		log.Info.Println("Start publishing job:", desc.ID.String())

		task := func() error {
			targetDir := "/cvmfs/" + desc.Repo + "/" + desc.Path
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				log.Error.Println("Could not create target dir:", err)
				return err
			}
			if err := getter.GetFile(targetDir, desc.Payload); err != nil {
				log.Error.Println("Could not download payload:", err)
				return err
			}
			return nil
		}

		if err := transaction.Run(desc, task); err != nil {
			log.Error.Println("Could not run CVMFS transaction:", err)
			j.Nack(false, true)
			continue
		}

		j.Ack(false)
		log.Info.Println("Finished publishing job:", desc.ID.String())
	}
}