package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetProjectIDfromURL(t *testing.T) {
	assert := assert.New(t)
	url := "https://api.github.com/projects/2640902"
	id, err := getProjectIDfromURL(url)
	assert.NoError(err)
	assert.EqualValues(2640902, id)
}

func TestGetProjectIDfromURLErr(t *testing.T) {
	assert := assert.New(t)
	url := ""
	_, err := getProjectIDfromURL(url)
	assert.EqualError(err, "invalid project url: ")

	url = "https://api.github.com/projects/"
	_, err = getProjectIDfromURL(url)
	assert.EqualError(err, "url was missing project id at end: https://api.github.com/projects/")

	url = "https://api.github.com/projects/abc"
	_, err = getProjectIDfromURL(url)
	assert.EqualError(err, "strconv.ParseInt: parsing \"abc\": invalid syntax")
}
