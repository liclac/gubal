package cmd

import (
	"encoding/json"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/liclac/gubal/fetcher"
)

// fetchCharCmd represents the fetch char command
var fetchCharCmd = &cobra.Command{
	Use:   "char [id] [num]",
	Short: "Queue up characters to be fetched",
	Long:  `Queue up characters to be fetched.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		startID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return err
		}
		endID := startID
		if len(args) > 1 {
			id, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				return err
			}
			endID = id
		}

		var bodies [][]byte
		for id := startID; id <= endID; id++ {
			job := fetcher.FetchCharacterJob{ID: id}
			msg := fetcher.FetchMessage{Job: job}
			body, err := json.Marshal(msg)
			if err != nil {
				return err
			}
			bodies = append(bodies, body)
		}

		p, err := newNSQProducer()
		if err != nil {
			return err
		}
		return p.MultiPublish(fetcher.FetchTopic, bodies)
	},
}

func init() {
	fetchCmd.AddCommand(fetchCharCmd)
}
