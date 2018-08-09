package integration

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"

	"github.com/cjsaylor/chessbot/game"
	"github.com/cjsaylor/slack"
	"github.com/cjsaylor/slack/slackevents"
	"github.com/notnil/chess"
)

type SlackHandler struct {
	BotToken          string
	VerificationToken string
	SigningKey        string
	Hostname          string
	SlackClient       *slack.Client
	GameStorage       game.GameStorage
	ChallengeStorage  game.ChallengeStorage
}

const requestVersion = "v0"

type command uint8

const (
	unknownCommand command = iota
	challengeCommand
	moveCommand
)

var commandPatterns = map[command]*regexp.Regexp{
	challengeCommand: regexp.MustCompile("^<@[\\w|\\d]+>.*challenge <@([\\w\\d]+)>.*$"),
	moveCommand:      regexp.MustCompile("^<@[\\w|\\d]+> .*([a-h][1-8][a-h][1-8]).*$"),
}

func (s SlackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)
	body := buf.String()

	event, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionVerifyToken(&slackevents.TokenComparator{s.VerificationToken}))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(err)
		return
	}
	if event.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal([]byte(body), &r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text")
		w.Write([]byte(r.Challenge))
	} else if event.Type == slackevents.CallbackEvent {
		if s.SlackClient == nil {
			s.SlackClient = slack.New(s.BotToken)
		}
		innerEvent := event.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			var gameID string
			if ev.ThreadTimeStamp == "" {
				gameID = ev.TimeStamp
			} else {
				gameID = ev.ThreadTimeStamp
			}
			handler := unknownCommand
			var captures []string
			for command, regex := range commandPatterns {
				results := regex.FindStringSubmatch(ev.Text)
				if len(results) > 0 {
					handler = command
					captures = results[1:]
				}
			}
			switch handler {
			case unknownCommand:
				s.sendError(gameID, ev.Channel, "Does not compute. :(")
			case moveCommand:
				s.handleMoveCommand(gameID, captures[0], ev)
			case challengeCommand:
				s.handleChallengeCommand(gameID, captures[0], ev)
			}
		}
	}
}

func (s SlackHandler) handleMoveCommand(gameID string, move string, ev *slackevents.AppMentionEvent) {
	gm, err := s.GameStorage.RetrieveGame(gameID)
	if err != nil {
		log.Println(err)
		gm = game.NewGame(game.Player{
			ID: ev.User,
		}, game.Player{
			ID: ev.User,
		})
		s.GameStorage.StoreGame(gameID, gm)
	}
	player := gm.TurnPlayer()
	if ev.User != player.ID {
		log.Println("ignoreing player input as it is not their turn")
	}
	chessMove, err := gm.Move(move)
	if err != nil {
		s.sendError(gameID, ev.Channel, err.Error())
		return
	}
	boardAttachment := slack.Attachment{
		Text:     chessMove.String(),
		ImageURL: fmt.Sprintf("%v/board?game_id=%v&ts=%v", s.Hostname, gameID, ev.EventTimeStamp),
	}
	if outcome := gm.Outcome(); outcome != "*" {
		if outcome == chess.Draw {
			s.SlackClient.PostMessage(ev.Channel, gm.ResultText(), slack.PostMessageParameters{
				ThreadTimestamp: ev.TimeStamp,
				Attachments:     []slack.Attachment{boardAttachment},
			})
		} else {
			var winningPlayer game.Player
			if outcome == chess.WhiteWon {
				winningPlayer = gm.Players[game.White]
			} else {
				winningPlayer = gm.Players[game.Black]
			}
			s.SlackClient.PostMessage(ev.Channel, fmt.Sprintf("Congratulation <@%v>!", winningPlayer.ID), slack.PostMessageParameters{
				ThreadTimestamp: ev.TimeStamp,
				Attachments:     []slack.Attachment{boardAttachment},
			})
			// @todo persist record to some incremental storage (redis, etc)
		}

		fmt.Println(gm.ResultText())
	} else {
		s.SlackClient.PostMessage(ev.Channel, fmt.Sprintf("<@%v>'s (%v) turn.", player.ID, gm.Turn()), slack.PostMessageParameters{
			ThreadTimestamp: ev.TimeStamp,
			Attachments:     []slack.Attachment{boardAttachment},
		})
	}
}

func (s SlackHandler) handleChallengeCommand(gameID string, challengedUser string, ev *slackevents.AppMentionEvent) {
	_, _, channelID, err := s.SlackClient.OpenIMChannel(challengedUser)
	if err != nil {
		log.Printf("unable to challenge %v: %v", challengedUser, err)
		s.sendError(gameID, ev.Channel, "Unable to challenge that player. :(")
		return
	}
	challenge := &game.Challenge{
		ChallengerID: ev.User,
		ChallengedID: challengedUser,
		GameID:       gameID,
		ChannelID:    ev.Channel,
	}
	s.ChallengeStorage.StoreChallenge(challenge)
	s.SlackClient.PostMessage(channelID, fmt.Sprintf("<@%v> has challenged you to a game of chess!", ev.User), slack.PostMessageParameters{
		Attachments: []slack.Attachment{
			slack.Attachment{
				Text:       "Do you accept?",
				Fallback:   "Unable to accept the challenge.",
				CallbackID: "challenge_response",
				Actions: []slack.AttachmentAction{
					slack.AttachmentAction{
						Name:  "challenge",
						Text:  "Accept Challenge",
						Type:  "button",
						Value: "accept",
					},
					slack.AttachmentAction{
						Name:  "challenge",
						Text:  "Decline",
						Type:  "button",
						Style: "danger",
						Value: "reject",
					},
				},
			},
		},
	})
}

func (s SlackHandler) sendError(gameID string, channel string, text string) {
	s.SlackClient.PostMessage(channel, text, slack.PostMessageParameters{
		ThreadTimestamp: gameID,
	})
}

// Not using this for now since the challenge request doesn't appear to send it
// Also we'd need to implement this in a form that slackevents.ParseEvent() can use for verification
func (s SlackHandler) validateSignature(r *http.Request, body string) bool {
	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	requestSignature := r.Header.Get("X-Slack-Signature")
	compiled := fmt.Sprintf("%v:%v:%v", requestVersion, timestamp, body)
	mac := hmac.New(sha256.New, []byte(s.SigningKey))
	mac.Write([]byte(compiled))
	expectedSignature := mac.Sum(nil)
	return hmac.Equal(expectedSignature, []byte(requestSignature))
}
