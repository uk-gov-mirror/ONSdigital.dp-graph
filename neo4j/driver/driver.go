package driver

import (
	"context"
	"io"

	"github.com/ONSdigital/dp-graph/graph/driver"
	"github.com/ONSdigital/dp-graph/neo4j/mapper"
	bolt "github.com/ONSdigital/golang-neo4j-bolt-driver"
	"github.com/pkg/errors"
)

//go:generate moq -out ../internal/driver.go -pkg internal . Neo4jDriver

type Neo4jDriver interface {
	Read(query string, mapp mapper.ResultMapper, single bool) error
	Count(query string) (count int64, err error)
	Exec(query string, params map[string]interface{}) error
	Close(ctx context.Context) error
	Healthcheck() (string, error)
}

type NeoDriver struct {
	pool bolt.ClosableDriverPool
}

func New(dbAddr string, size, timeout int) (n *NeoDriver, err error) {
	pool, err := bolt.NewClosableDriverPoolWithTimeout(dbAddr, size, timeout)
	if err != nil {
		return nil, err
	}

	return &NeoDriver{
		pool: pool,
	}, nil
}

func (n *NeoDriver) Close(ctx context.Context) error {
	return n.pool.Close()
}

func (n *NeoDriver) Read(query string, mapp mapper.ResultMapper, single bool) error {
	c, err := n.pool.OpenPool()
	if err != nil {
		return err
	}
	defer c.Close()

	rows, err := c.QueryNeo(query, nil)
	if err != nil {
		return errors.WithMessage(err, "error executing neo4j query")
	}
	defer rows.Close()

	index := 0
	numOfResults := 0
results:
	for {
		data, meta, nextNeoErr := rows.NextNeo()
		if nextNeoErr != nil {
			if nextNeoErr != io.EOF {
				return errors.WithMessage(nextNeoErr, "extractResults: rows.NextNeo() return unexpected error")
			}
			break results
		}

		numOfResults++
		if single && index > 0 {
			return errors.WithMessage(err, "non unique results")
		}

		if mapp != nil {
			if err := mapp(&mapper.Result{Data: data, Meta: meta, Index: index}); err != nil {
				return errors.WithMessage(err, "mapResult returned an error")
			}
		}
		index++
	}

	if numOfResults == 0 {
		return driver.ErrNotFound
	}

	return nil
}

func (n *NeoDriver) Count(query string) (count int64, err error) {
	c, err := n.pool.OpenPool()
	if err != nil {
		return
	}
	defer c.Close()

	rows, err := c.QueryNeo(query, nil)
	if err != nil {
		err = errors.WithMessage(err, "error executing neo4j query")
		return
	}
	defer rows.Close()

	data, _, err := rows.All()
	if err != nil {
		return
	}

	var ok bool
	if count, ok = data[0][0].(int64); !ok {
		err = errors.New("Could not get result from DB")
	}

	return
}

func (n *NeoDriver) Exec(query string, params map[string]interface{}) error {
	c, err := n.pool.OpenPool()
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err := c.ExecNeo(query, params); err != nil {
		return errors.WithMessage(err, "error executing neo4j query")
	}

	return nil
}
