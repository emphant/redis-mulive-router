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

	//r := &Point{1,2}
	//fmt.Println(r)
	//defer_call()

	//var b []byte = nil
	//
	//log.Println(string(b))
	//v := mySqrt(1)
	//math.Sqrt(10)
	//fmt.Printf("return is %s",v)
	image := [][]int{{1,1,1},{1,1,0},{1,0,1}}
	print2D(image)
	sr := 1
	sc := 1
	newColor := 2
	arr := floodFill(image,sr,sc,newColor)
	fmt.Println(arr)
	print2D(arr)

}
func print2D(arr [][]int){
	for _,y := range arr{
		for _,j := range y  {
			fmt.Printf("\t%d",j)
		}
		fmt.Print("\n")
	}
}

type Point struct {
	X,Y float64
}

func defer_call()  {
	defer func() {
		fmt.Println("1")
	}()
	defer func() {
		fmt.Println("2")
	}()
	defer func() {
		fmt.Println("3")
	}()
	panic("error")
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


func mySqrt(x int) int {
	return gsq(x,x,0)
}
func gsq(x int,start int,end int) int {
	half := (start+end)/2
	if half*half<x {
		if start==end {
			return start
		}else if (half+1)*(half+1)>x {
			return half
		}else if (half+1)*(half+1)==x {
			return half+1
		}else {
			return gsq(x,start,half)
		}
	}else if half*half==x{
		return half
	}else {
		return gsq(x,half,0)
	}
}

func floodFill(image [][]int, sr int, sc int, newColor int) [][]int {
	if image[sr][sc]==newColor {
		return image
	}
	oldColor:=image[sr][sc]
	rlen := len(image)
	clen := len(image[0])
	floodFill2(image[:][:],sr,sc,rlen,clen,oldColor,newColor)
	return image[:][:]
}

func floodFill2(image [][]int, sr , sc ,rn ,cn ,oldColor, newColor int)  {
	if sr<0 || sr >=rn || sc<0 || sc >=cn {
		return
	}
	if image[sr][sc] != oldColor {
		return
	}
	image[sr][sc]=newColor
	floodFill2(image[:][:],sr+1,sc,rn,cn,oldColor,newColor)
	floodFill2(image[:][:],sr,sc+1,rn,cn,oldColor,newColor)
	floodFill2(image[:][:],sr-1,sc,rn,cn,oldColor,newColor)
	floodFill2(image[:][:],sr,sc-1,rn,cn,oldColor,newColor)
}
