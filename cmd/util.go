package cmd

import (
	"log"
	"strings"

	"github.com/jinzhu/gorm"
	nsq "github.com/nsqio/go-nsq"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func dbConnect() (*gorm.DB, error) {
	uri := viper.GetString("db")
	parts := strings.SplitN(uri, "://", 2)
	if len(parts) != 2 {
		return nil, errors.New("db string lacks schema")
	}
	db, err := gorm.Open(parts[0], uri)
	if err != nil {
		return nil, err
	}
	if !viper.GetBool("prod") {
		db = db.LogMode(true).Debug()
	}
	return db, nil
}

type nsqLogAdapter struct{}

func (nsqLogAdapter) Output(calldepth int, s string) error { return log.Output(calldepth, s) }

func newNSQProducer() (*nsq.Producer, error) {
	p, err := nsq.NewProducer(viper.GetString("nsqd"), nsq.NewConfig())
	if err != nil {
		return nil, err
	}
	p.SetLogger(nsqLogAdapter{}, nsq.LogLevelDebug)
	return p, nil
}

func newNSQConsumer(topic, channel string) (*nsq.Consumer, error) {
	c, err := nsq.NewConsumer(topic, channel, nsq.NewConfig())
	if err != nil {
		return nil, err
	}
	c.SetLogger(nsqLogAdapter{}, nsq.LogLevelDebug)
	return c, nil
}

func nsqConsumerConnect(c *nsq.Consumer) error {
	return c.ConnectToNSQLookupd(viper.GetString("nsqlookupd"))
}
