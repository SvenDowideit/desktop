// +build !nacl,!plan9

package verboselog

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"os"
)

// Verboselog Hook writes all levels of logrus entries to a file for later analysis
type VerboselogHook struct {
	Writer        *os.File
}

func NewVerboselogHook(filename string) (*VerboselogHook, error) {
	f, err := os.Create(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to open %s log file, %v", filename, err)
		return nil, err
	}

	return &VerboselogHook{f}, err
}

func (hook *VerboselogHook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read entry, %v", err)
		return err
	}

	_, err = hook.Writer.WriteString(line)
	return err
}

func (hook *VerboselogHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.DebugLevel,
		logrus.InfoLevel,
		logrus.WarnLevel,
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}
}
