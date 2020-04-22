package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cjsaylor/chessbot/game"
	"github.com/cjsaylor/chessbot/integration"
)

func randomInt(min, max int) int {
	return min + rand.Intn(max-min)
}

// Generate a random string of A-Z chars with len = l
func randomString(len int) string {
	bytes := make([]byte, len)
	for i := 0; i < len; i++ {
		bytes[i] = byte(randomInt(97, 122))
	}
	return string(bytes)
}

const (
	export integration.CommandType = iota + integration.Help
	fen
	exit
)

func main() {
	var cloudcube_cube = os.Getenv("CLOUDCUBE_CUBENAME")
	var cloudcube_bucket = "cloud-cube-eu"

	// Session object should be reused
	sess, serr := session.NewSession(&aws.Config{
		Region: aws.String("eu-west-1")},
	)
	if serr != nil {
		fmt.Println(serr)
		os.Exit(1)
	}

	// Create S3 service client and uploader
	svc := s3.New(sess)
	keyName := cloudcube_cube + "/chessbot.db"
	uploader := s3manager.NewUploaderWithClient(svc)

	// Download the current db file
	downloader := s3manager.NewDownloaderWithClient(svc)
	f, ferr := os.Create("./chessbot.db")
	if ferr != nil {
		fmt.Println(ferr)
	}

	_, derr := downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(cloudcube_bucket),
		Key:    aws.String(keyName),
	})
	if derr != nil {
		fmt.Printf("error downloading the db file!")
		fmt.Println(derr)
	}

	rand.Seed(time.Now().UnixNano())
	fmt.Println("Game REPL")
	fmt.Println("Note: piece colors may appear reversed on dark background terminals.")
	gameID := "constantGameId"
	store, _ := game.NewSqliteStore("./chessbot.db", uploader, cloudcube_bucket, keyName)
	fmt.Println("Game ID: " + gameID)
	var gm *game.Game
	players := []game.Player{
		game.Player{ID: "player1"},
		game.Player{ID: "player2"},
	}
	gm, gerr := store.RetrieveGame(gameID)
	if gerr != nil {
		fmt.Println(gerr)
		fmt.Println("Creating a new game...")
		gm = game.NewGame(gameID, players...)

	}

	inputParser := integration.NewCommandParser([]integration.CommandPattern{
		{
			Type:    integration.Move,
			Pattern: regexp.MustCompile("^.*([a-h][1-8][a-h][1-8][qnrb]?).*$"),
		},
		{
			Type:    integration.Resign,
			Pattern: regexp.MustCompile("^.*resign.*"),
		},
		{
			Type:    integration.Takeback,
			Pattern: regexp.MustCompile("^.*takeback.*"),
		},
		{
			Type:    export,
			Pattern: regexp.MustCompile("^.*export.*$"),
		},
		{
			Type:    fen,
			Pattern: regexp.MustCompile("^.*fen.*$"),
		},
		{
			Type:    exit,
			Pattern: regexp.MustCompile("^.*exit.*$"),
		},
	})
	store.StoreGame(gameID, gm)
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println(gm)
	fmt.Printf("%v's turn (%v)\n", gm.TurnPlayer().ID, gm.Turn())
	fmt.Print("\n> ")
	for scanner.Scan() {
		input := scanner.Text()
		matchedCommand := inputParser.ParseInput(input)
		gm, err := store.RetrieveGame(gameID)
		if err != nil {
			fmt.Println("Error reading in game: ", err)
		}
		switch matchedCommand.Type {
		case fen:
			fmt.Println(gm.FEN())
			fmt.Print("\n> ")
			continue
		case export:
			fmt.Println(gm.Export())
			fmt.Print("\n> ")
			continue
		case exit:
			os.Exit(0)
		case integration.Move:
			moveCommand, err := matchedCommand.ToMove()
			if err != nil {
				fmt.Println(err)
				fmt.Print("\n> ")
				continue
			}
			_, err = gm.Move(moveCommand.LAN)
			if err != nil {
				fmt.Println(err)
				fmt.Print("\n> ")
				continue
			}
		case integration.Resign:
			gm.Resign(gm.TurnPlayer())
		case integration.Takeback:
			currentTurnPlayer := gm.TurnPlayer()
			var takebackPlayer game.Player
			for _, player := range players {
				if player.ID != currentTurnPlayer.ID {
					takebackPlayer = player
				}
			}
			if _, err := gm.Takeback(&takebackPlayer); err != nil {
				fmt.Println(err)
				fmt.Print("\n> ")
				continue
			}
		}

		store.StoreGame(gameID, gm)
		fmt.Println(gm)
		if outcome := gm.Outcome(); outcome != "*" {
			fmt.Println(gm.ResultText())
			os.Exit(0)
		}
		fmt.Printf("%v's turn (%v)\n", gm.TurnPlayer().ID, gm.Turn())
		fmt.Print("\n> ")
	}

}
