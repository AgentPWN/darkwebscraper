package main

import (
	"darkwebscraper/website"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	funcs := []func(query string) bool{
		website.Breachforums,
		website.Dread,
		website.Lockbit,
		website.LeakBase,
	}
	var wg sync.WaitGroup
	contents, err := os.ReadFile("names.txt")
	if err != nil {
		panic(err)
	} else {
		eachContent := strings.SplitSeq(string(contents), "\n")
		for i := range eachContent {
			// fmt.Println(i)
			i = strings.TrimSpace(i)
			for _, f := range funcs {
				wg.Add(1)
				go func(fn func(query string) bool) {
					defer wg.Done()
					fn(i)
				}(f)
			}
			wg.Wait()
			time.Sleep(5 * time.Second)
		}
	}
}
