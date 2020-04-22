package integration

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"regexp"

	"github.com/cjsaylor/chessbot/game"
	"github.com/cjsaylor/chessbot/rendering"
	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
)

var challengerPattern = regexp.MustCompile("^<@([\\w|\\d]+).*$")

// SlackActionHandler will respond to all Slack integration component requests
type SlackActionHandler struct {
	SigningKey   string
	Hostname     string
	SlackClient  *slack.Client
	AuthStorage  AuthStorage
	GameStorage  game.GameStorage
	LinkRenderer rendering.RenderLink
}

func (s SlackActionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)
	body := buf.String()

	if len(body) < 8 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	secretsVerifier, err := slack.NewSecretsVerifier(r.Header, s.SigningKey)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	secretsVerifier.Write([]byte(body))
	if err := secretsVerifier.Ensure(); err != nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	payload, _ := url.QueryUnescape(body[8:])
	event, err := slackevents.ParseActionEvent(payload, slackevents.OptionNoVerifyToken())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(err)
		return
	}
	if s.SlackClient == nil {
		botToken, err := s.AuthStorage.GetAuthToken(event.Team.ID)
		if err != nil {
			log.Panicln(err)
		}
		s.SlackClient = slack.New(botToken)
	}
	if event.Type != "interactive_message" && event.CallbackID != "challenge_response" {
		s.sendResponse(w, event.OriginalMessage, "Invalid action.")
		return
	}
}

func (s SlackActionHandler) sendResponse(w http.ResponseWriter, original slack.Message, text string) {
	original.ReplaceOriginal = true
	original.Attachments[0].Actions = []slack.AttachmentAction{}
	original.Attachments[0].Fields = []slack.AttachmentField{
		{
			Title: text,
			Value: "",
			Short: false,
		},
	}
	w.Header().Add("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&original)
}
