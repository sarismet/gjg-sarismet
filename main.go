package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/gjg-sarismet/endpoints"
)

func main() {
	sync_wanted := false
	var interval int
	fmt.Println("os.Args : ", len(os.Args))
	if len(os.Args) > 1 {
		if os.Args[1] == "-sync" {
			if len(os.Args) > 2 && os.Args[2] == "-i" {
				if len(os.Args) > 3 {
					intervalNo, err := strconv.Atoi(os.Args[3])
					if err == nil {
						interval = intervalNo
						sync_wanted = true
					} else {
						fmt.Println("interval number is not integer")
						os.Exit(127)
					}
				} else {
					fmt.Println("interval number is not set or empty")
					os.Exit(127)
				}
			} else {
				fmt.Println("interval argment '-i' is not found")
				os.Exit(127)
			}
		} else {
			fmt.Println("sync arguments were not set properly")
			os.Exit(127)
		}
	}

	fmt.Println("sync_wanted : ", sync_wanted, " interval ", interval)
	endpoints.Init(sync_wanted, interval)

}
