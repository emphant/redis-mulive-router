package main

import (
	"fmt"
	//"bufio"
	//"os"
	"time"
	"strconv"
)
//TODO merge URL.path 到主干分支

func main() {
	//EmpByID(10).Name="fefe"

	//fmt.Print("Enter your text: ")
	//qu := getQu()
	//
	//reader := makeReder()
	//for r := range reader {
	//	qu <- r
	//}

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


	//f(3)

	r := &Point{1,2}
	fmt.Println(r)
}

type Point struct {
	X,Y float64
}



func f(x int){
	fmt.Printf("f(%d) \n",x+0/x)
	defer fmt.Printf("defer %d\n",x)
	f(x-1)
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

type Employee struct {
	ID int
	Name string
}

func EmpByID(id int) *Employee{
	e   := Employee{id,"name"}
	return &e
}

//EmpByID(10).Name="fefe"