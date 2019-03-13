package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/cvmfs/conveyor/internal/cvmfs"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

type checkCmdVars struct {
	ids      *[]string
	repo     string
	wait     *bool
	extended *bool
}

var chkvs checkCmdVars

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "check job status",
	Long:  "check the status of a submitted job",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := cvmfs.ReadConfig()
		if err != nil {
			cvmfs.Log.Error().Err(err).Msg("config error")
			os.Exit(1)
		}
		if rootCmd.PersistentFlags().Changed("timeout") {
			cfg.JobWaitTimeout = jobWaitTimeout
		}

		client, err := cvmfs.NewJobClient(cfg)
		if err != nil {
			cvmfs.Log.Error().Err(err).Msg("could not start job client")
			os.Exit(1)
		}

		// Optionally wait for completion of the jobs
		if *chkvs.wait {
			_, err := client.WaitForJobs(*chkvs.ids, chkvs.repo, jobWaitTimeout)
			if err != nil {
				cvmfs.Log.Error().Err(err).Msg("waiting for job completion failed")
				os.Exit(1)
			}
		}

		quit := make(chan struct{})
		stats, err := client.GetJobStatus(*chkvs.ids, chkvs.repo, *chkvs.extended, quit)
		if err != nil {
			cvmfs.Log.Error().Err(err).Msg("error checking job status")
			os.Exit(1)
		}

		if stats.Status != "ok" {
			cvmfs.Log.Error().Err(errors.New(stats.Reason)).Msg("job failed")
			os.Exit(1)
		}

		cvmfs.Log.Info().Msg("Completed jobs:")
		if *chkvs.extended {
			for _, j := range stats.Jobs {
				printStatus(j.ID, j)
			}
		} else {
			for _, j := range stats.IDs {
				printStatus(j.ID, j)
			}
		}
	},
}

func printStatus(id uuid.UUID, st interface{}) {
	buf, err := json.Marshal(&st)
	if err != nil {
		cvmfs.Log.Error().Err(err).
			Str("job_id", id.String()).
			Msg("job status JSON serialization error")
	} else {
		fmt.Println(string(buf))
	}
}

func init() {
	chkvs.ids = checkCmd.Flags().StringSlice(
		"ids", []string{}, "comma-separate list of job UUIDs to query")
	checkCmd.MarkFlagRequired("ids")
	checkCmd.Flags().StringVar(&chkvs.repo, "repo", "", "target CVMFS repository of the jobs ")
	checkCmd.MarkFlagRequired("repo")
	chkvs.wait = checkCmd.Flags().Bool("wait", false, "wait for completion of the queried jobs")
	chkvs.extended = checkCmd.Flags().Bool("extended-status", false, "return the extended status of the job")
}
