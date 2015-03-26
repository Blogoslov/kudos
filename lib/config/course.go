package config

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/synful/kudos/lib/log"
)

const (
	CourseConfigDirName  = ".kudos"
	CourseConfigFileName = "config.toml"
)

type HandinDir string

func (h *HandinDir) UnmarshalTOML(i interface{}) error {
	path, ok := i.(string)
	if !ok {
		return fmt.Errorf("expected string value")
	}
	path = filepath.Clean(path)
	if filepath.IsAbs(path) {
		return fmt.Errorf("must be relative path")
	}
	*h = HandinDir(path)
	return nil
}

type HandinMethod string

const (
	FaclMethod   HandinMethod = "facl"
	SetgidMethod HandinMethod = "setgid"
)

func (h *HandinMethod) UnmarshalTOML(i interface{}) error {
	method, _ := i.(string)
	hmethod := HandinMethod(strings.ToLower(method))
	if hmethod != FaclMethod && hmethod != SetgidMethod {
		return fmt.Errorf("allowed methods: %v, %v", FaclMethod, SetgidMethod)
	}
	*h = hmethod
	return nil
}

type CourseConfig struct {
	Name             string       `toml:"name"`
	TaGroup          string       `toml:"ta_group"`
	StudentGroup     string       `toml:"student_group"`
	HandinDir        HandinDir    `toml:"handin_dir"`
	HandinMethod     HandinMethod `toml:"handin_method"`
	ShortDescription string       `toml:"short_description"`
	LongDescription  string       `toml:"long_description"`
}

func (c CourseConfig) WriteTOML(w io.Writer) (err error) {
	return toml.NewEncoder(w).Encode(&c)
}

func DefaultCourseConfig() CourseConfig {
	return CourseConfig{
		Name:             "cs101",
		TaGroup:          "cs101tas",
		StudentGroup:     "cs101students",
		HandinDir:        HandinDir("handin"),
		HandinMethod:     FaclMethod,
		ShortDescription: "CS 101",
		LongDescription:  "This is an introductory course in CS.",
	}
}

func ReadCourseConfig(course, coursePath string) (CourseConfig, error) {
	confPath := filepath.Join(coursePath, CourseConfigDirName, CourseConfigFileName)
	log.Debug.Printf("reading course config file: %v\n", confPath)
	var conf CourseConfig
	_, err := toml.DecodeFile(confPath, &conf)
	if err != nil {
		return CourseConfig{}, fmt.Errorf("could not parse course config: %v", err)
	}
	if course != conf.Name {
		return CourseConfig{}, fmt.Errorf("course name in config (%v) does not match expected name (%v)", conf.Name, course)
	}
	return conf, nil
}
