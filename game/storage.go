package game

// GameStorage is an interface to be implemented for persisting a game
type GameStorage interface {
	RetrieveGame(ID string) (*Game, error)
	StoreGame(ID string, game *Game) error
}
