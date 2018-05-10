package main

import (
	"fmt"
	//"bufio"
	//"os"
	"time"
	"strconv"
)


func main() {
	fmt.Print("Enter your text: ")
	qu := getQu()

	reader := makeReder()
	for r := range reader {
		qu <- r
	}

	//scanner := bufio.NewScanner(os.Stdin)
	//fmt.Print("Enter your text: ")
	//for scanner.Scan() {
	//	info :=scanner.Text()
	//	qu <- info
	//}
	//if scanner.Err() != nil {
	//	// handle error.
	//}


	//info := "user"
	//qu <- info

}
func makeReder() chan string {
	tasks := make(chan string)
	go genInfos(tasks)
	return tasks
}
func genInfos(tasks chan string) {

	for r := range [1000]int{} {
		fmt.Println(r)
		tasks <- strconv.Itoa(r)
		time.Sleep(1*time.Second)
	}
}

func getQu() chan<- string  {
	tasks := make(chan string)
	go handle(tasks)
	return tasks
}
func handle(tasks chan string) {
	for r := range tasks {
		fmt.Printf("you wrote %v \r\n",r)
	}
	fmt.Println("handle finish")
}
