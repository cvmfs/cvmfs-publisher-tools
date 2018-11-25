package jobdb

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/stdlib" // Import and register the PostgreSQL driver

	"github.com/cvmfs/cvmfs-publisher-tools/internal/job"
	"github.com/cvmfs/cvmfs-publisher-tools/internal/log"
	"github.com/pkg/errors"
)

// BackendConfig - database backend configuration for the job db service
type BackendConfig struct {
	Type     string
	Database string
	Username string
	Password string
	Host     string
	Port     int
}

// Backend - encapsulates the backend state
type Backend struct {
	db *sql.DB
}

// Close - closes the database connection
func (b *Backend) Close() {
	b.db.Close()
}

// GetJob - returns the row from the job DB corresponding to the ID
func (b *Backend) GetJob(id string, full bool) (*job.GetJobReply, error) {
	reply := job.GetJobReply{Status: "ok", Reason: ""}

	rows, err := b.db.Query("select * from Jobs where ID = $1", id)
	if err != nil {
		reply.Status = "error"
		reply.Reason = "query error"
		return &reply, errors.Wrap(err, "query failed")
	}
	defer rows.Close()

	if !rows.Next() {
		return &reply, nil
	}

	st, err := scanRow(rows)
	if err != nil {
		reply.Status = "error"
		reply.Reason = "query failed"
		return &reply, errors.Wrap(err, "scan failed")
	}

	if full {
		reply.Jobs = []job.Processed{*st}
	} else {
		reply.IDs = []job.Status{job.Status{ID: st.ID, Successful: st.Successful}}
	}

	return &reply, nil
}

// GetJobs - returns the rows from the job DB corresponding to the IDs
func (b *Backend) GetJobs(ids []string, full bool) (*job.GetJobReply, error) {
	reply := job.GetJobReply{Status: "ok", Reason: ""}

	queryStr := "select * from Jobs where Jobs.ID in ("
	params := make([]interface{}, len(ids))
	for i, v := range ids[0 : len(ids)-1] {
		queryStr += fmt.Sprintf("$%v, ", i+1)
		params[i] = v
	}
	queryStr += fmt.Sprintf("$%v);", len(ids))
	params[len(ids)-1] = ids[len(ids)-1]

	rows, err := b.db.Query(queryStr, params...)
	if err != nil {
		reason := "SQL query failed"
		reply.Status = "error"
		reply.Reason = reason
		return &reply, errors.Wrap(err, reason)
	}
	defer rows.Close()

	for rows.Next() {
		st, err := scanRow(rows)
		if err != nil {
			reason := "SQL query scan failed"
			reply.Status = "error"
			reply.Reason = reason
			reply.IDs = []job.Status{}
			reply.Jobs = []job.Processed{}
			return &reply, errors.Wrap(err, reason)
		}

		if full {
			reply.Jobs = append(reply.Jobs, *st)
		} else {
			reply.IDs = append(reply.IDs, job.Status{ID: st.ID, Successful: st.Successful})
		}
	}

	return &reply, nil
}

// PutJob - inserts a job into the DB
func (b *Backend) PutJob(j *job.Processed) (*job.PutJobReply, error) {
	reply := job.PutJobReply{Status: "ok", Reason: ""}

	tx, err := b.db.Begin()
	if err != nil {
		reason := "opening SQL transaction failed"
		reply.Status = "error"
		reply.Reason = reason
		return &reply, errors.Wrap(err, reason)
	}
	defer tx.Rollback()

	queryStr := "insert into jobs (ID,Repository,Payload,RepositoryPath,Script,ScriptArgs," +
		"TransferScript,Dependencies,StartTime,FinishTime,Successful,ErrorMessage) " +
		"values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12);"
	if _, err := tx.Exec(queryStr,
		j.ID, j.Repository, j.Payload, j.RepositoryPath,
		j.Script, j.ScriptArgs, j.TransferScript, strings.Join(j.Dependencies, ","),
		j.StartTime, j.FinishTime, j.Successful, j.ErrorMessage); err != nil {
		reason := "executing SQL statement failed"
		reply.Status = "error"
		reply.Reason = reason
		return &reply, errors.Wrap(err, reason)
	}
	err = tx.Commit()
	if err != nil {
		reason := "committing SQL transaction failed"
		reply.Status = "error"
		reply.Reason = reason
		return &reply, errors.Wrap(err, reason)
	}

	log.Info.Printf(
		"Job inserted: %v, success: %v, start time: %v, finish time: %v\n",
		j.ID, j.Successful, j.StartTime, j.FinishTime)

	return &reply, nil
}

func scanRow(rows *sql.Rows) (*job.Processed, error) {
	var st job.Processed
	var deps string
	if err := rows.Scan(
		&st.ID, &st.Repository, &st.Payload, &st.RepositoryPath,
		&st.Script, &st.ScriptArgs, &st.TransferScript,
		&deps, &st.StartTime, &st.FinishTime,
		&st.Successful, &st.ErrorMessage); err != nil {
		return nil, err
	}
	if deps != "" {
		st.Dependencies = strings.Split(deps, ",")
	}

	return &st, nil
}

func startBackEnd(cfg BackendConfig) (*Backend, error) {
	db, err := sql.Open("pgx", createDataSrcName(cfg))
	if err != nil {
		return nil, errors.Wrap(err, "could not create SQL connection")
	}

	if err := db.Ping(); err != nil {
		return nil, errors.Wrap(err, "connection ping failed")
	}

	return &Backend{db}, nil
}

func createDataSrcName(cfg BackendConfig) string {
	return fmt.Sprintf(
		"user=%s password=%s host=%s port=%v dbname=%s sslmode=disable",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
}
