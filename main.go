package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

type pool struct {
	connections chan *sql.DB
}


func simulateDbOperation(connection *sql.DB) error{
	_,err := connection.Exec("select sleep(0.01);")
	return err
}




func benchmarkNonPool(dbConnections int, dsn string) (time.Duration, error) {
	start := time.Now()
	var(
		wg sync.WaitGroup
		operr error
		once sync.Once
	)
	
	helper := func(){
		defer wg.Done()
		// create a new connection for each query

		connection, err := sql.Open("mysql",dsn)
		if err != nil {
			once.Do(func(){operr = err})
			return

		}

		connection.SetMaxOpenConns(1)
		connection.SetMaxIdleConns(1)

		defer connection.Close()

		err = simulateDbOperation(connection)

		if err != nil {
			once.Do(func(){operr = err})
			return
		}



	}


	for i := 0;i<dbConnections;i++ {
		wg.Add(1)
		go helper()
	}

	wg.Wait()


	timeDuration := time.Since(start)

	return timeDuration, operr

}

func benchmarkPool(dbConnections int, pool *pool) (time.Duration, error) {
	start := time.Now()
	var(
		wg sync.WaitGroup
		operr error
		once sync.Once
	)
	
	helper := func(){

		defer wg.Done()

		connection := pool.acquire()

		defer pool.release(connection)

		err := simulateDbOperation(connection)

		if err != nil {
			once.Do(func(){ operr = err})
		}

		

	}


	for i := 0; i<dbConnections; i++ {
		wg.Add(1)
		go helper()
	}

	wg.Wait()

	timeDuration := time.Since(start)

	return timeDuration, operr



}

func newPool(maxConnections int, dsn string) (*pool, error) {
	p := &pool{
		connections: make(chan *sql.DB, maxConnections),
	}

	for i := 0; i < maxConnections; i++ {
		connection, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil,err
		}

		// ensuring there is only single connections underlying to each connection object
		connection.SetMaxOpenConns(1)
		connection.SetMaxIdleConns(1)

		// verify that the connection to the database server is live and the credentials are valid
		if err := connection.Ping(); err != nil {
			log.Fatal(err)
		}

		p.connections <- connection

	}
	return p,nil
}


func (pool *pool) acquire() *sql.DB{
	return <-pool.connections
}

func (pool *pool) release(connection *sql.DB) {
	pool.connections <- connection
}

func (pool *pool) close() {
	close(pool.connections)
	for connection := range pool.connections {
		connection.Close()
	}
}

func main() {

	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASS"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	tests := []int{10,20,30,40,50,75,100,150,200,300,400}

	// Benchmark Non Pool connections
	fmt.Println("Starting non pool benchmarks")

	for _, dbConnections := range tests {
		elapsed, err := benchmarkNonPool(dbConnections, dsn)
		if err != nil {
			log.Fatal(err)
		} else {
			fmt.Println("No. of db connections: ", dbConnections, "time elapsed without pooling connections: ", elapsed)
		}

	}

	// Benchmark Pool connections
	fmt.Println("starting connection pooling benchmarks")
	maxConnections := 50 // no. of connections to the database provided in the pool
	pool, err := newPool(maxConnections, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.close()
	for _, dbConnections := range tests {
		elapsed, err := benchmarkPool(dbConnections, pool)
		if err != nil {
			log.Fatal(err)
		} else {
			fmt.Println("No. of db Connections: ", dbConnections, "Time elapsed when pooling connections: ", elapsed)
		}

	}

}
