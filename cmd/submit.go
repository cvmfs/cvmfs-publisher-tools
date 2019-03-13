package commands

import (
	"errors"
	"os"

	"github.com/cvmfs/conveyor/internal/cvmfs"
	"github.com/spf13/cobra"
)

type submitCmdVars struct {
	jobName        string
	repo           string
	payload        string
	path           string
	script         string
	scriptArgs     string
	transferScript *bool
	deps           *[]string
	wait           *bool
}

var subvs submitCmdVars

var submitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit a job",
	Long:  "Submit a publishing job to a queue",
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

		spec := &cvmfs.JobSpecification{
			JobName: subvs.jobName, Repository: subvs.repo, Payload: subvs.payload,
			RepositoryPath: subvs.path, Script: subvs.script, ScriptArgs: subvs.scriptArgs, TransferScript: *subvs.transferScript, Dependencies: *subvs.deps}

		if err := spec.Prepare(); err != nil {
			cvmfs.Log.Error().Err(err).Msg("could not create job object")
			os.Exit(1)
		}

		client, err := cvmfs.NewJobClient(cfg)
		if err != nil {
			cvmfs.Log.Error().Err(err).Msg("could not start job client")
			os.Exit(1)
		}

		stat, err := client.PostNewJob(spec)
		if err != nil {
			cvmfs.Log.Error().Err(err).Msg("could not post new job")
			os.Exit(1)
		}

		if stat.Status != "ok" {
			cvmfs.Log.Error().
				Err(errors.New(stat.Reason)).
				Msg("job failed")
			os.Exit(1)
		}

		id := stat.ID

		// Optionally wait for completion of the job
		if *subvs.wait {
			stats, err := client.WaitForJobs(
				[]string{id.String()}, spec.Repository, jobWaitTimeout)
			if err != nil {
				cvmfs.Log.Error().
					Err(err).
					Msg("waiting for job completion failed")
				os.Exit(1)
			}

			if !stats[0].Successful {
				cvmfs.Log.Error().
					Str("job_id", id.String()).
					Msg("job dependency failed")
				os.Exit(1)
			}
		}

		cvmfs.Log.Info().Str("job_id", id.String()).Msg("job submitted successfully")
	},
}

func init() {
	submitCmd.Flags().StringVar(&subvs.jobName, "job-name", "", "name of the job")
	submitCmd.Flags().StringVar(&subvs.repo, "repo", "", "target CVMFS repository")
	submitCmd.MarkFlagRequired("repo")
	submitCmd.Flags().StringVar(&subvs.payload, "payload", "", "payload URL")
	submitCmd.Flags().StringVar(&subvs.path, "path", "/", "target path inside the repository")
	submitCmd.Flags().StringVar(
		&subvs.script, "script", "", "script to run at the end of CVMFS transaction")
	submitCmd.Flags().StringVar(
		&subvs.scriptArgs, "script-args", "", "arguments of the transaction script")
	subvs.transferScript = submitCmd.Flags().Bool(
		"transfer-script", false, "transaction script is a local file which should be sent")
	subvs.deps = submitCmd.Flags().StringSlice(
		"deps", []string{}, "comma-separate list of job dependency UUIDs")
	subvs.wait = submitCmd.Flags().Bool("wait", false, "wait for completion of the submitted job")
}
