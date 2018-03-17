package cmd

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"path"
	"sync"
	"time"

	"github.com/nsqio/go-nsq"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/liclac/gubal/fetcher"
	"github.com/liclac/gubal/lib"
	"github.com/liclac/gubal/models"
)

// fetcherCmd represents the fetcher command
var fetcherCmd = &cobra.Command{
	Use:   "fetcher",
	Short: "Run a fetcher process",
	Long:  `Run a fetcher process.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var wg sync.WaitGroup
		defer wg.Wait()

		concurrency := viper.GetInt("concurrency")
		zap.L().Info("Starting fetcher...", zap.Int("concurrency", concurrency))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Prepare a cache...
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		cacheFS := afero.NewBasePathFs(afero.NewOsFs(), path.Join(wd, "cache"))
		ctx = fetcher.WithCacheFS(ctx, cacheFS)

		// Connect to the database...
		db, err := dbConnect()
		if err != nil {
			return err
		}
		defer db.Close()

		// Adjust pool size to concurrency.
		db.DB().SetMaxOpenConns(concurrency * 2)
		db.DB().SetMaxIdleConns(concurrency * 2)

		// Connect to NSQ...
		prod, err := newNSQProducer()
		if err != nil {
			return err
		}
		cons, err := newNSQConsumer(fetcher.FetchTopic, "fetcher")
		if err != nil {
			return err
		}
		cons.ChangeMaxInFlight(concurrency)
		cons.AddConcurrentHandlers(nsq.HandlerFunc(func(m *nsq.Message) (rerr error) {
			wg.Add(1)
			defer wg.Done()

			// TODO: Stop tying datastores to database instances, this is really wasteful.
			// You have the context right there, just use it >_>
			tx := db.Begin()
			defer func() {
				if rerr != nil {
					rerr = multierr.Append(rerr, tx.Rollback().Error)
				} else {
					rerr = multierr.Append(rerr, tx.Commit().Error)
				}
			}()
			ctx = lib.WithRawDB(ctx, tx)
			ctx = models.WithDataStore(ctx, models.NewDataStore(db))

			zap.L().Debug("Processing...",
				zap.ByteString("body", m.Body),
				zap.Time("time", time.Unix(0, m.Timestamp)),
				zap.Uint16("attempts", m.Attempts),
			)
			var msg fetcher.FetchMessage
			if err := json.Unmarshal(m.Body, &msg); err != nil {
				return err
			}

			jobs, err := msg.Job.Run(ctx)
			if err != nil {
				return err
			}
			switch len(jobs) {
			case 0:
				return nil
			case 1:
				data, err := json.Marshal(fetcher.FetchMessage{Job: jobs[0]})
				if err != nil {
					return err
				}
				return prod.Publish(fetcher.FetchTopic, data)
			default:
				bodies := make([][]byte, len(jobs))
				for i, job := range jobs {
					data, err := json.Marshal(fetcher.FetchMessage{Job: job})
					if err != nil {
						return err
					}
					bodies[i] = data
				}
				return prod.MultiPublish(fetcher.FetchTopic, bodies)
			}
		}), concurrency)
		if err := nsqConsumerConnect(cons); err != nil {
			return err
		}
		defer func() {
			cons.Stop()
			<-cons.StopChan
		}()

		// Wait for a signal, then cancel the connection and wait.
		sigC := make(chan os.Signal, 1)
		signal.Notify(sigC)
		signal.Stop(sigC)
		<-sigC
		cancel()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(fetcherCmd)
	fetcherCmd.Flags().IntP("concurrency", "c", 10, "concurrent jobs to process")
	must(viper.BindPFlags(fetcherCmd.Flags()))
}
