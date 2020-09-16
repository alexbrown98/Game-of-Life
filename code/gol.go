package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ChrisGora/semaphore"
)

type commandsStruct struct {
	pPressed chan bool
	qPressed chan bool
	sPressed chan bool
}

func key_actions(key_chan chan rune, p_pressed, s_pressed, q_pressed chan bool) {
	for {
		key := <-key_chan
		if key == 'p' {
			fmt.Println("P pressed")
			p_pressed <- true
		} else if key == 's' {
			s_pressed <- true
		} else if key == 'q' {
			q_pressed <- true
		}
	}
}

func pause_execution(turns int, ticker_cells *TickerCells) {
	fmt.Println("Paused. Current turn in execution: ", turns)
	ticker_cells.paused = true
	bufio.NewReader(os.Stdin).ReadBytes('p')
	ticker_cells.paused = false
	fmt.Println("Continuing.")
}
func printPgm(p golParams, d distributorChans, finalAlive []cell, i int) {
	d.io.command <- ioOutput
	d.io.filename <- strconv.Itoa(p.imageHeight) + "x" + strconv.Itoa(p.imageWidth) + "Turn " + strconv.Itoa(i)
	d.io.cells_alive <- finalAlive
}

func workerManager(p golParams, wg *sync.WaitGroup, splits []int, original_world, world [][]byte, w commandsStruct, d distributorChans) {
	var wg1, wg2 sync.WaitGroup
	var sem, sem1 semaphore.Semaphore

	sem = semaphore.Init(p.threads, 0)
	sem1 = semaphore.Init(p.threads, 0)
	// Create a new ticker
	ticker := time.NewTicker(2 * time.Second)

	//Create a new TickerCells struct to communicate the number of cells alive
	var m sync.RWMutex
	ticker_cells := TickerCells{mu: m, last: 0, paused: false}

	//Create the ticker function that uses a RWMutex to print the current number of alive cells
	go func(ticker_cells *TickerCells) {
		for {
			select {
			case <-ticker.C:
				if !ticker_cells.paused {
					ticker_cells.mu.Lock()
					fmt.Println("Cells alive: ", ticker_cells.last)
					ticker_cells.mu.Unlock()
				}
			}
		}

	}(&ticker_cells)

	for i := 0; i < p.threads; i++ {
		if i == 0 {
			wg1.Add(1)
			wg2.Add(1)
			go worker(0, splits[i], original_world, world, p, &wg1, &wg2, sem, sem1)
		} else {
			wg1.Add(1)
			wg2.Add(1)
			go worker((splits[i-1]), splits[i], original_world, world, p, &wg1, &wg2, sem, sem1)
		}
	}
	for turns := 0; turns < p.turns; turns++ {
		var exit bool = false
		select {
		case pBool := <-w.pPressed:
			if pBool {
				pause_execution(turns, &ticker_cells)
			} else {
			}
		case qBool := <-w.qPressed:
			if qBool {
				exit = true
				fmt.Println("Execution stopped at turn: ", turns)
			}
		case sBool := <-w.sPressed: //When s is pressed, a pgm must be outputed without halting execution of
			if sBool { // the worker functions, hence a unique flow of execution must be provided
				cellsAlive := []cell{}
				for y := 0; y < p.imageHeight; y++ {
					for x := 0; x < p.imageWidth; x++ {
						if world[y][x] != 0 {
							cellsAlive = append(cellsAlive, cell{x, y})
						}
					}
				}
				printPgm(p, d, cellsAlive, turns)
			}
		default:
		}
		if exit {
			break
		}
		totAlive := 0

		wg2.Wait()
		if turns != 0 {
			wg1.Add(p.threads)
			for y := 0; y < p.imageHeight; y++ {
				for x := 0; x < p.imageWidth; x++ {
					original_world[y][x] = world[y][x]
					if original_world[y][x] != 0 {
						totAlive++
					}
				}
			}
		}
		for j := 0; j < p.threads; j++ {
			wg2.Add(1)
			sem.Post()
		}
		ticker_cells.mu.Lock()
		ticker_cells.last = totAlive
		ticker_cells.mu.Unlock()
		wg1.Wait()
		for j := 0; j < p.threads; j++ {
			sem1.Post()
		}
	}

	wg.Done()
}

func worker(start int, end int, original_world [][]byte, world [][]byte, p golParams, wg1, wg2 *sync.WaitGroup, sem, sem1 semaphore.Semaphore) {
	for i := 0; i < p.turns; i++ {
		wg2.Done()
		sem.Wait()

		for y := start; y < end; y++ {
			for x := 0; x < p.imageWidth; x++ {

				var prev_row, next_row, prev_col, next_col int

				if x > 0 {
					prev_col = x - 1
				} else {
					prev_col = p.imageWidth - 1
				}

				if x < p.imageWidth-1 {
					next_col = x + 1
				} else {
					next_col = 0
				}

				if y > 0 {
					prev_row = y - 1
				} else {
					prev_row = p.imageHeight - 1
				}

				if y < p.imageHeight-1 {
					next_row = y + 1
				} else {
					next_row = 0
				}

				var is_alive bool
				var tot_alive int = 0
				if original_world[y][x] != 0 {
					is_alive = true
				} else {
					is_alive = false
				}
				if original_world[y][prev_col] != 0 {
					tot_alive++
				}
				if original_world[y][next_col] != 0 {
					tot_alive++
				}
				if original_world[prev_row][prev_col] != 0 {
					tot_alive++
				}
				if original_world[prev_row][x] != 0 {
					tot_alive++
				}
				if original_world[prev_row][next_col] != 0 {
					tot_alive++
				}
				if original_world[next_row][prev_col] != 0 {
					tot_alive++
				}
				if original_world[next_row][x] != 0 {
					tot_alive++
				}
				if original_world[next_row][next_col] != 0 {
					tot_alive++
				}
				if is_alive && tot_alive > 3 {
					world[y][x] = 0x00
				}
				if is_alive && tot_alive < 2 {
					world[y][x] = 0x00
				}
				if !is_alive && tot_alive == 3 {
					world[y][x] = 0xFF
				}
			}
		}
		wg1.Done()
		sem1.Wait()
	}

}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p golParams, d distributorChans, alive chan []cell) {

	// Create the 2D slice to store the world.
	world := make([][]byte, p.imageHeight)

	for i := range world {
		world[i] = make([]byte, p.imageWidth)
	}

	// Request the io goroutine to read in the image with the given filename.
	d.io.command <- ioInput
	name_file := strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")
	d.io.filename <- name_file

	// The io goroutine sends the requested image byte by byte, in rows.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			val := <-d.io.inputVal
			if val != 0 {
				fmt.Println("Alive cell at", x, y)
				world[y][x] = val
			}
		}
	}

	p_pressed := make(chan bool)
	q_pressed := make(chan bool)
	s_pressed := make(chan bool)
	key_chan := make(chan rune)
	go key_actions(key_chan, p_pressed, s_pressed, q_pressed)
	go getKeyboardCommand(key_chan)
	wChans := commandsStruct{p_pressed, q_pressed, s_pressed}

	//Initialise a helper array to hard code the worker splits
	no_workers := p.threads
	splits := make([]int, no_workers)
	if p.imageHeight%p.threads == 0 { // For worker threads equal to some power of 2
		for i := 1; i < p.threads+1; i++ {
			splits[i-1] = p.imageHeight / p.threads * i
		}
	} else {
		for i := 1; i < p.threads; i++ { //Worker threads not equal to some power of 2 (works for odds)
			splits[i-1] = p.imageHeight / p.threads * i
		}
		splits[p.threads-1] = p.imageHeight
	}

	//Create a copy of the current world
	original_world := make([][]byte, p.imageHeight)
	for i := range world {
		original_world[i] = make([]byte, p.imageWidth)
	}
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			original_world[y][x] = world[y][x]
		}
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go workerManager(p, &wg, splits, original_world, world, wChans, d)
	wg.Wait()

	// Create an empty slice to store coordinates of cells that are still alive after p.turns are done.
	var finalAlive []cell
	// Go through the world and append the cells that are still alive.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			if world[y][x] != 0 {
				finalAlive = append(finalAlive, cell{x: x, y: y})
			}
		}
	}

	// IO commands for output
	d.io.command <- ioOutput
	d.io.filename <- name_file
	d.io.cells_alive <- finalAlive

	// Make sure that the Io has finished any output before exiting.
	d.io.command <- ioCheckIdle
	<-d.io.idle

	// Return the coordinates of cells that are still alive.
	alive <- finalAlive

}
