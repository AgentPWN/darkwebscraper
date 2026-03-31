package website

import "fmt"

func LeakBase(query string) bool {
	fmt.Println(query)
	return true
}
