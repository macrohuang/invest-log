package handlers

import (
	"net/http"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
)

func TransactionsHandler(c *gin.Context) {
	// For now, just render with empty data or simple query
	render(c, "transactions.html", pongo2.Context{
		"transactions": []interface{}{},
	})
}

func render(c *gin.Context, name string, data pongo2.Context) {
	tmpl, err := pongo2.FromFile("templates/" + name)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	err = tmpl.ExecuteWriter(data, c.Writer)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}
}
