package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/cjsaylor/chessbot/config"
	"github.com/cjsaylor/chessbot/game"
	"github.com/cjsaylor/chessbot/integration"
	"github.com/cjsaylor/chessbot/rendering"
)

func main() {
	config, err := config.ParseConfiguration()
	if err != nil {
		log.Fatal(err)
	}
	var gameStorage game.GameStorage
	var challengeStorage game.ChallengeStorage
	var authStorage integration.AuthStorage
	if config.SqlitePath != "" {
		sqlStore, err := game.NewSqliteStore(config.SqlitePath)
		if err != nil {
			log.Fatal(err)
		}
		lruSqlStore, err := game.NewSqliteLRUStore(sqlStore, config.GameCacheSize, func() {
			os.Exit(1)
		})
		authSqlStore, err := integration.NewSqliteStore(config.SqlitePath)
		gameStorage = lruSqlStore
		challengeStorage = sqlStore
		authStorage = authSqlStore
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
	http.Handle("/slack", integration.SlackHandler{
		VerificationToken: config.SlackVerificationToken,
		Hostname:          config.Hostname,
		AuthStorage:       authStorage,
		GameStorage:       gameStorage,
		ChallengeStorage:  challengeStorage,
		LinkRenderer:      renderLink,
	})
	http.Handle("/slack/action", integration.SlackActionHandler{
		VerificationToken: config.SlackVerificationToken,
		Hostname:          config.Hostname,
		AuthStorage:       authStorage,
		GameStorage:       gameStorage,
		ChallengeStorage:  challengeStorage,
		LinkRenderer:      renderLink,
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
