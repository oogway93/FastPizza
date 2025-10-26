package main

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/oogway93/FastPizza/utils"
)

func fibonacciHandler(c *gin.Context) {
	n := c.Query("n")
	N, err := strconv.Atoi(n)
	utils.FailOnError(err)
	c.JSON(200, map[string]int{"result": N})

}

func main() {
	r := gin.Default()
	r.GET("/fib", fibonacciHandler)
	r.Run()
}

