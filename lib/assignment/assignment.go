package assignment

import (
	"io/ioutil"
	"time"

	"github.com/BurntSushi/toml"
)

func timeparse(text string) (time.Time, error) {
	return time.Parse("Jan 2, 2006 at 3:04pm (MST)", text)
}

type HandinMetadata struct {
	// TODO(synful)
}

type Problem struct {
	Name  string
	Files []string
	Total int
}

type AssignSpec struct {
	Title   string
	Problem []Problem
	Handin  Handin
}

type Handin struct {
	Due   date
	Grace duration
}

type date struct {
	time.Time
}

func (d *date) UnmarshalText(text []byte) error {
	var err error
	d.Time, err = timeparse(string(text))
	return err
}

// TAKEN from burntsushi example
type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

//end taken from example

func AsgnFromFile(file string) (AssignSpec, error) {
	var fileText []byte
	var err error
	var res AssignSpec

	if fileText, err = ioutil.ReadFile(file); err != nil {
		return AssignSpec{}, err
	}

	if _, err = toml.Decode(string(fileText), &res); err != nil {
		return AssignSpec{}, err
	}

	return res, nil
}