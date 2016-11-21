// +build !nacl,!plan9

package showuserlog

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"os"
)

// Showuserlog Hook writes all levels of logrus entries to a file for later analysis
type ShowuserlogHook struct {
	Level logrus.Level
}

func NewShowuserlogHook(l logrus.Level) (*ShowuserlogHook, error) {
	return &ShowuserlogHook{l}, nil
}

func (hook *ShowuserlogHook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read entry, %v", err)
		return err
	}

	if entry.Level <= hook.Level {
		_, ok := entry.Data["cmd"]
		if ok {
			fmt.Printf(".")
		} else {
			if entry.Level == logrus.InfoLevel {
				fmt.Println(entry.Message)
			} else {
				fmt.Println(line)
			}
		}
	}
	return nil
}

func (hook *ShowuserlogHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.DebugLevel,
		logrus.InfoLevel,
		logrus.WarnLevel,
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}
}
