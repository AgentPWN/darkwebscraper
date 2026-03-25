package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

func breachForums(query string) {
	fmt.Println(query)
}
func dread(query string) {
	fmt.Println(query)
}
func lockBit(query string) {
	fmt.Println(query)
}
func leakBase(query string) {
	fmt.Println(query)
}
func main() {
	funcs := []func(query string){
		breachForums,
		dread,
		lockBit,
		leakBase,
	}
	var wg sync.WaitGroup
	fmt.Println("hello world")
	contents, err := os.ReadFile("names.txt")
	if err == nil {
		eachContent := strings.SplitSeq(string(contents), "\n")
		for i := range eachContent {
			// fmt.Println(i)
			for _, f := range funcs {
				wg.Add(1)
				go func(fn func(query string)) {
					defer wg.Done()
					fn(i)
				}(f)
			}
			wg.Wait()
		}
	}
}
