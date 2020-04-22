package game_test

import (
	"errors"
	"testing"

	"github.com/cjsaylor/chessbot/game"
)

type dbTest struct {
	name string
	db   game.GameStorage
}

func dbTestTable() ([]dbTest, error) {
	/*sqlite, err := game.NewSqliteStore("../chessbot.db")
	if err != nil {
		return []dbTest{}, nil
	}
	memory := game.NewMemoryStore()*/
	return []dbTest{}, errors.New("err")
}

func TestGameSavesAndIsRetrievable(t *testing.T) {
	dbSet, err := dbTestTable()
	if err != nil {
		t.Error(err)
	}
	for _, tt := range dbSet {
		t.Run(tt.name, func(t *testing.T) {
			gm := game.NewGame("1234", game.Player{ID: "1"}, game.Player{ID: "2"})
			if err := tt.db.StoreGame("1234", gm); err != nil {
				t.Error(err)
			}
			gm, err = tt.db.RetrieveGame("1234")
			if err != nil {
				t.Error(err)
			}
		})
	}
}

func TestGameSavesLastMoved(t *testing.T) {
	dbSet, err := dbTestTable()
	if err != nil {
		t.Error(err)
	}
	for _, tt := range dbSet {
		t.Run(tt.name, func(t *testing.T) {
			gm := game.NewGame("1234", game.Player{ID: "1"}, game.Player{ID: "2"})
			gm.Move("d2d4")
			if err := tt.db.StoreGame("1234", gm); err != nil {
				t.Error(err)
			}
			gm, err = tt.db.RetrieveGame("1234")
			if err != nil {
				t.Error(err)
			}
			if gm.LastMoved().IsZero() {
				t.Error("expected the last moved time to be set")
			}
		})
	}
}

func TestRetrieveGameThatDoesNotExist(t *testing.T) {
	dbSet, err := dbTestTable()
	if err != nil {
		t.Error(err)
	}
	for _, tt := range dbSet {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.db.RetrieveGame("NOEXISTID"); err == nil {
				t.Error("should throw and error because the game was not found")
			}
		})
	}

}
