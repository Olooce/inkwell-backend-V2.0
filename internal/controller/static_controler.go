package controller

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type StaticController struct{}

func NewStaticController() *StaticController {
	return &StaticController{}
}

func (sc *StaticController) ServeStatic(c *gin.Context) {
	c.FileFromFS(c.Request.RequestURI, http.Dir("./working"))
}

func (sc *StaticController) DownloadComic(c *gin.Context) {
	filename := c.Param("filename")
	filePath := "./working/comics/" + filename
	if filepath.Ext(filename) == ".pdf" {
		c.Header("Content-Disposition", "attachment; filename="+filename)
		c.Header("Content-Type", "application/pdf")
	}
	c.File(filePath)
}
