package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cjsaylor/chessbot/analysis"
	"github.com/cjsaylor/chessbot/config"
	"github.com/cjsaylor/chessbot/game"
	"github.com/cjsaylor/chessbot/integration"
	"github.com/cjsaylor/chessbot/rendering"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	dir, dderr := os.Getwd()
	if dderr != nil {
		log.Fatal(dderr)
	}
	fmt.Println("Current dir: " + dir)

	// AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY set as env
	var cloudcube_cube = os.Getenv("CLOUDCUBE_CUBENAME")
	var cloudcube_bucket = os.Getenv("cloud-cube-eu")

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

	config, err := config.ParseConfiguration()
	if err != nil {
		log.Fatal(err)
	}
	var gameStorage game.GameStorage
	var challengeStorage game.ChallengeStorage
	var authStorage integration.AuthStorage
	if config.SqlitePath != "" {
		gameSQLStore, err := game.NewSqliteStore(config.SqlitePath, uploader, cloudcube_bucket, keyName)
		if err != nil {
			log.Fatal(err)
		}
		authSQLStore, err := integration.NewSqliteStore(config.SqlitePath)
		gameStorage = gameSQLStore
		challengeStorage = gameSQLStore
		authStorage = authSQLStore
	} else {
		memoryStore := game.NewMemoryStore()
		gameStorage = memoryStore
		challengeStorage = memoryStore
		authStorage = integration.NewMemoryStore()
	}
	renderLink := rendering.NewRenderLink(config.Hostname, config.SigningKey)
	http.Handle("/board", rendering.BoardRenderHandler{
		LinkRenderer: renderLink,
	})
	http.Handle("/analyze", analysis.NewHTTPHandler(gameStorage, analysis.NewChesscomAnalyzer(config.ChessAffiliateCode)))
	http.Handle("/slack", integration.SlackHandler{
		SigningKey:       config.SlackSigningKey,
		Hostname:         config.Hostname,
		AuthStorage:      authStorage,
		GameStorage:      gameStorage,
		ChallengeStorage: challengeStorage,
		LinkRenderer:     renderLink,
	})
	http.Handle("/slack/action", integration.SlackActionHandler{
		SigningKey:       config.SlackSigningKey,
		Hostname:         config.Hostname,
		AuthStorage:      authStorage,
		GameStorage:      gameStorage,
		ChallengeStorage: challengeStorage,
		LinkRenderer:     renderLink,
	})
	http.Handle("/slack/oauth", integration.SlackOauthHandler{
		SlackClientID:     config.SlackClientID,
		SlackClientSecret: config.SlackClientSecret,
		SlackAppID:        config.SlackAppID,
		AuthStore:         authStorage,
	})
	log.Printf("Listening on port %v\n", config.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", config.Port), nil))
}
