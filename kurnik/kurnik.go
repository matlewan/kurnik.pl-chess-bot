package kurnik

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"main/uci"
	"main/utils"

	"github.com/notnil/chess"

	"github.com/thanhpk/randstr"

	"github.com/gorilla/websocket"
)

var x = []byte("abcdefgh")
var y = []byte("87654321")

var promotionOptions = []byte("qnbr")

const userAgent = `Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:66.0) Gecko/20100101 Firefox/66.0`

const (
	MatchResultWin  MatchResult = 0
	MatchResultLoss MatchResult = 1
	MatchResultDraw MatchResult = 2
)

const (
	GameStateInGame GameState = 8
	GameStateLost   GameState = 1
	GameStateWon    GameState = 0
	GameStateDraw   GameState = 9
)

type PlayerList map[string]User
type RoomList map[int]Room
type GameState int
type MatchResult int

type BotSettings struct {
	Account struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	} `json:"account"`
	EngineDepth   int               `json:"engine_depth"`
	EnginePath    string            `json:"engine_path"`
	AutoStartGame bool              `json:"auto_start"`
	KickIfLowElo  bool              `json:"kick_low_elo"`
	KickIfLose    bool              `json:"kick_if_lose"`
	KickIfDraw    bool              `json:"kick_if_draw"`
	EngineOptions map[string]string `json:"engine_options"`
}

type PayloadInt struct {
	I []int `json:"i"`
}

type PayloadIntString struct {
	I []int    `json:"i"`
	S []string `json:"s"`
}

type User struct {
	Name   string
	N      int
	RoomID int
	Rating int
}

type Player struct {
	User           User
	CurrentSection string
	CurrentSeat    int
	RatingChange   []int
}

type KurnikBot struct {
	Connection     *websocket.Conn
	CurrentPlayer  Player
	CurrentSection string
	RoomList       RoomList
	SectionsList   []string
	PlayerList     PlayerList
	Game           Game
	Engine         *uci.ChessEngine
	BotSettings    BotSettings
	WebClients     WebClientList
	MatchHistory   []Match
	LastGameState  GameState
	Running        bool
}

type Match struct {
	Player      User
	Opponent    User
	MarchResult MatchResult
	EloChange   int
	PGN         string
}

type Game struct {
	Turn      int
	Chess     *chess.Game
	IsWhite   bool
	EloChange EloChange
}

type Seat struct {
	Player User
	Taken  bool
}

type OpenedRoom struct {
	Base       Room
	PlayerList PlayerList
}

type Room struct {
	N      int
	InGame bool
	Time   string
	Seats  map[int]Seat
}

type Move struct {
	BestMove    string
	From        string
	To          string
	isPromotion bool
}

type EloChange struct {
	Win  int
	Loss int
	Draw int
}

// K=32
func CalculateEloChange(e1, e2 int) EloChange {
	diff := float64(e2 - e1)
	precentage := float64(1 / (1 + math.Pow(10, diff/400)))
	return EloChange{
		int(math.Round(32 * (1 - precentage))),
		int(math.Round(32 * (.5 - precentage))),
		int(math.Round(32 * (precentage))),
	}
}

func (q *KurnikBot) NewRoomObject(i []int, s []string) Room {
	r := Room{}
	r.Seats = make(map[int]Seat)

	r.N = i[0]
	if i[1] == 1 {
		r.InGame = true
	}

	r.Time = s[0]

	for j := 0; j < 2; j++ {
		seat := Seat{}
		if i[2+j] == 1 {
			seat.Taken = true
		}
		if s[1+j] != "" {
			seat.Player = q.PlayerList[s[1+j]]
		}
		r.Seats[j] = seat
	}

	return r
}

func BuildLoginPayload(sessionID string) PayloadIntString {
	if sessionID == "" {
		sessionID = randstr.String(16)
	}
	p := PayloadIntString{
		[]int{1710},
		[]string{
			sessionID + "+" + randstr.String(18, "1234567890") + "||",
			"en",
			"b",
			"",
			userAgent,
			fmt.Sprintf("/%d/1", time.Now().Unix()*1000),
			"w",
			"1366x768 1",
			"ref:https://www.kurnik.pl/szachy/",
			"ver:191",
		},
	}
	return p
}

func (q *KurnikBot) SendKeepAlive() {
	p := PayloadInt{[]int{2}}
	q.SendMessage(&p)
}

func (q *KurnikBot) LeaveRoom(room int) {
	p := PayloadInt{[]int{73, room}}
	q.CurrentPlayer.User.RoomID = 0
	q.SendMessage(&p)
}

func (q *KurnikBot) StartMatch() {
	p := PayloadInt{[]int{85, q.CurrentPlayer.User.RoomID}}
	q.SendMessage(&p)
}

func (q *KurnikBot) JoinRoom(roomID int) {
	p := PayloadInt{[]int{72, roomID}}
	q.CurrentPlayer.User.RoomID = roomID
	q.SendMessage(&p)
}

func (q *KurnikBot) SendChatMessage(message string) {
	p := PayloadIntString{[]int{81, q.CurrentPlayer.User.RoomID}, []string{message}}
	q.SendMessage(&p)
}

func (q *KurnikBot) CreateRoom() {
	p := PayloadInt{[]int{71}}
	q.SendMessage(&p)
}

func (q *KurnikBot) TakeSeat(seat int) {
	p := PayloadInt{[]int{83, q.CurrentPlayer.User.RoomID, seat}}
	q.SendMessage(&p)
}

func (q *KurnikBot) ConnectToWebSocketServer() {
	u := url.URL{Scheme: "wss", Host: "x.kurnik.pl:17003", Path: "/ws/"}

	headers := make(http.Header)
	headers.Set("Cookie", "kt=cckn")
	headers.Set("User-Agent", userAgent)
	headers.Set("Origin", "http://kurnik.pl/")

	var err error
	q.Connection, _, err = websocket.DefaultDialer.Dial(u.String(), headers)
	if err != nil {
		panic(err)
	}
	q.Running = true
}

func (q *KurnikBot) Exit() error {
	q.Running = false
	err := q.Engine.Close()
	err = q.Connection.Close()
	return err
}

func (q *KurnikBot) StartListening() {
	for q.Running {
		var p PayloadIntString
		_, b, err := q.Connection.ReadMessage()
		if err != nil {
			return
		}
		if len(b) > 0 {
			err := json.Unmarshal(b, &p)
			if err != nil {
				panic(err)
			}
			q.HandleCommands(p)
		}
	}
}

func (q *KurnikBot) BroadcastWebSocketMessage(p *WebPayload) {
	for _, v := range q.WebClients {
		WebSocketWriteJson(v, p)
	}
}

func WebSocketWriteJson(conn *websocket.Conn, p *WebPayload) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, b)
}

func (q *KurnikBot) HandleWebSocketMessage(p WebPayload, conn *websocket.Conn) {
	switch p.Command {
	case "init_rating":
		wp := WebPayload{}
		wp.Command = "add_rating"
		wp.Data = q.CurrentPlayer.RatingChange

		WebSocketWriteJson(conn, &wp)
	case "depth":
		v, ok := p.Data.(int)
		if ok {
			q.BotSettings.EngineDepth = v
		}
	case "kick_low_elo":
		v, ok := p.Data.(bool)
		if ok {
			q.BotSettings.KickIfLowElo = v
		}
	case "auto_start":
		v, ok := p.Data.(bool)
		if ok {
			q.BotSettings.AutoStartGame = v
		}
	case "create_room":
		q.CreateRoom()
	case "join_section":
		v, ok := p.Data.(string)
		if ok {
			found := false
			for _, s := range q.SectionsList {
				if s == v {
					found = true
					break
				}
			}
			if found {
				q.JoinSection(v)
			}
		}
	}
}

func (q *KurnikBot) HandleCommands(p PayloadIntString) {
	switch p.I[0] {
	case 1:
		q.SendKeepAlive()
	case 18:
		q.ReceiveUsername(p)
	case 24:
		q.HandlePlayerLeave(p)
	case 25:
		q.HandlePlayerUpdate(p)
	case 27:
		q.ReceivePlayerList(p)
	case 32:
		q.ReceiveSectionsList(p)
	case 33:
		q.ReceiveRating(p)
	case 70:
		q.ReceiveRoomUpdate(p)
	case 71:
		q.ReceiveRoomList(p)
	case 73:
		q.RecieveCurrentRoom(p)
	case 88:
		q.RecieveRoomSeat(p)
	case 90:
		q.RecievePossibleMoves(p)
	case 91:
		q.HandleStartGame(p)
	case 92:
		q.ReceiveMove(p)
	}
}

func (q *KurnikBot) KickPlayerFromRoom(name string) {
	p := PayloadIntString{[]int{81, q.CurrentPlayer.User.RoomID}, []string{"/boot " + name}}
	q.SendMessage(&p)
}

func (q *KurnikBot) KickEnemyFromSeat() {
	seatToKick := 1
	if q.CurrentPlayer.CurrentSeat == 1 {
		seatToKick = 0
	}
	p := PayloadInt{[]int{84, q.CurrentPlayer.User.RoomID, seatToKick}}
	q.SendMessage(&p)
}

func (q *KurnikBot) GetMoveFromEngine() (Move, error) {
	m := Move{}
	err := q.Engine.SetFEN(q.Game.Chess.FEN())
	if err != nil {
		return m, err
	}

	res, err := q.Engine.Depth(q.BotSettings.EngineDepth)
	if err != nil {
		return m, err
	}

	m.BestMove = res.BestMove
	m.From = res.BestMove[:2]
	m.To = res.BestMove[2:4]

	if len(res.BestMove) > 4 {
		m.isPromotion = true
	}
	return m, err
}

func CalculateMove(m Move) int {
	p0 := utils.IndexByte(m.From[0], x)
	p1 := utils.IndexByte(m.From[1], y)
	d0 := utils.IndexByte(m.To[0], x)
	d1 := utils.IndexByte(m.To[1], y)

	r := ((d1*8+d0)*8+p1)*8 + p0

	if m.isPromotion {
		r = (utils.IndexByte(m.BestMove[4], promotionOptions)+1)*4096 + r
	}
	return r
}

func (q *KurnikBot) SendMove(m Move, time int64) {
	sp := PayloadInt{}
	sp.I = []int{92, q.CurrentPlayer.User.RoomID, 1, CalculateMove(m), int(time)}
	q.SendMessage(&sp)
}

func (q *KurnikBot) RecieveCurrentRoom(p PayloadIntString) {
	q.CurrentPlayer.User.RoomID = p.I[1]
}

func (q *KurnikBot) RecievePossibleMoves(p PayloadIntString) {
	q.Game.Turn = p.I[3]
	gs := GameState(p.I[4])

	if q.Game.Turn != -1 &&
		q.CurrentPlayer.CurrentSeat == q.Game.Turn &&
		gs == GameStateInGame {
		start := time.Now()

		m, err := q.GetMoveFromEngine()
		if err != nil {
			panic(err)
		}

		elapsed := time.Since(start)
		t := elapsed.Nanoseconds() / 100000000
		if t <= 0 {
			t = 1
		}

		q.SendMove(m, t)

	} else if q.LastGameState == GameStateInGame {

		m := Match{}
		m.Player = q.CurrentPlayer.User
		m.Opponent = q.GetCurrentEnemy()
		m.PGN = q.Game.Chess.String()

		switch gs {
		case GameStateWon:
			m.MarchResult = MatchResultWin
			m.EloChange = q.Game.EloChange.Win

		case GameStateLost:
			m.MarchResult = MatchResultLoss
			m.EloChange = q.Game.EloChange.Loss

			if q.BotSettings.KickIfLose && q.LastGameState == GameStateInGame {
				q.KickPlayerFromRoom(q.GetCurrentEnemy().Name)
			}

		case GameStateDraw:
			m.MarchResult = MatchResultDraw
			m.EloChange = q.Game.EloChange.Draw

			if q.BotSettings.KickIfDraw && q.Game.EloChange.Draw > 0 && q.LastGameState == GameStateInGame {
				q.KickPlayerFromRoom(q.GetCurrentEnemy().Name)
			}
		}
		q.MatchHistory = append(q.MatchHistory, m)
	}
	q.LastGameState = gs
}

func (q *KurnikBot) RecieveRoomSeat(p PayloadIntString) {
	q.CurrentPlayer.CurrentSeat = p.I[4]
}

func (q *KurnikBot) ReceiveMove(p PayloadIntString) {
	err := q.Game.Chess.MoveStr(p.S[0])
	if err != nil {
		panic(err)
	}
}

func (q *KurnikBot) HandleStartGame(p PayloadIntString) {
	q.Game.Chess = chess.NewGame(chess.UseNotation(chess.AlgebraicNotation{}))
	if len(p.I) == 2 {
		q.Game.IsWhite = true
	} else {
		q.Game.IsWhite = false
	}
	q.Game.EloChange = q.GetEloChange()
}

func (q *KurnikBot) GetCurrentRoom() Room {
	return q.RoomList[q.CurrentPlayer.User.RoomID]
}

func (q *KurnikBot) ReceiveSectionsList(p PayloadIntString) {
	q.SectionsList = make([]string, 0)
	sp := strings.Split(p.S[0], "\n")
	for _, v := range sp {
		q.SectionsList = append(q.SectionsList, strings.Split(v, " ")[0])
	}
	q.CurrentSection = q.SectionsList[p.I[1]]
	fmt.Println(q.CurrentSection)
}

func (q *KurnikBot) HandlePlayerUpdate(p PayloadIntString) {
	u := User{}
	u.Rating = p.I[3]
	u.RoomID = p.I[2]
	u.N = p.I[1]
	u.Name = p.S[0]

	q.PlayerList[p.S[0]] = u
	if q.CurrentPlayer.User.Name == u.Name {
		q.CurrentPlayer.User = u
	}
}

func (q *KurnikBot) HandlePlayerLeave(p PayloadIntString) {
	delete(q.PlayerList, p.S[0])
}

func (q *KurnikBot) ReceiveCreateRoom(p PayloadIntString) {
	q.CurrentPlayer.User.RoomID = p.I[1]
}

func (q *KurnikBot) ReceiveRoomList(p PayloadIntString) {
	q.RoomList = make(RoomList)

	j := 0
	for i := 3; i < len(p.I)-3; i += 4 {
		r := q.NewRoomObject(p.I[i:i+4], p.S[j:j+3])

		q.RoomList[i] = r

		j += 3
	}
}

func (q *KurnikBot) GetCurrentEnemy() User {
	room := q.GetCurrentRoom()

	if q.CurrentPlayer.CurrentSeat == 0 {
		if room.Seats[1].Taken {
			return room.Seats[1].Player
		}
	}
	return room.Seats[0].Player
}

func (q *KurnikBot) GetEloChange() EloChange {
	room := q.GetCurrentRoom()

	e1 := room.Seats[0].Player.Rating
	e2 := room.Seats[1].Player.Rating

	return CalculateEloChange(e1, e2)
}

func (q *KurnikBot) ReceiveRoomUpdate(p PayloadIntString) {
	r := q.NewRoomObject(p.I[1:5], p.S[0:3])
	q.RoomList[r.N] = r

	if r.N == q.CurrentPlayer.User.RoomID {
		if r.Seats[0].Taken && r.Seats[1].Taken {
			change := q.GetEloChange()

			if q.BotSettings.KickIfLowElo && change.Win <= 0 {
				q.KickEnemyFromSeat()
			} else if q.BotSettings.AutoStartGame {
				q.StartMatch()
			}
		}
	}
}

func (q *KurnikBot) RecieveRemoveRoom(p PayloadIntString) {
	delete(q.RoomList, p.I[1])
}

func (q *KurnikBot) ReceiveUsername(p PayloadIntString) {
	q.CurrentPlayer.User.Name = p.S[0]
}

func (q *KurnikBot) ReceiveRating(p PayloadIntString) {
	q.CurrentPlayer.User.Rating = p.I[1]

	if len(q.CurrentPlayer.RatingChange) != 0 {
		if q.CurrentPlayer.RatingChange[len(q.CurrentPlayer.RatingChange)-1] == p.I[1] {
			return
		}
	}
	q.CurrentPlayer.RatingChange = append(q.CurrentPlayer.RatingChange, p.I[1])

	wp := WebPayload{"add_rating", []int{p.I[1]}}
	q.BroadcastWebSocketMessage(&wp)
}

func (q *KurnikBot) JoinSection(section string) {
	p := PayloadIntString{}
	p.I = []int{20}
	p.S = []string{"/join " + section}

	q.SendMessage(&p)
}

func (q *KurnikBot) ReceivePlayerList(p PayloadIntString) {
	q.PlayerList = make(PlayerList)

	n := 3
	for _, name := range p.S {
		player := User{}
		player.Name = name
		player.N = p.I[n]
		player.RoomID = p.I[n+1]
		player.Rating = p.I[n+2]
		q.PlayerList[name] = player
	}
}

func (q *KurnikBot) Login(login, password string) {
	p := BuildLoginPayload(GetSessionID(login, password))
	q.SendMessage(&p)
}

func (q *KurnikBot) LoginAsGuest() {
	p := BuildLoginPayload("")
	q.SendMessage(&p)
}

func (q *KurnikBot) SendMessage(v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	err = q.Connection.WriteMessage(1, b)
	if err != nil {
		panic(err)
	}
}

func GetSessionID(login, password string) string {
	body := "cc=0&username=" + login + "&pw=" + password
	req, err := http.NewRequest("POST", "https://www.kurnik.pl/login.phtml", strings.NewReader(body))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// parse cookie
	// 61 =
	// 58 :
	cookies := resp.Request.Response.Header["Set-Cookie"][1]
	var cookie = strings.Split(strings.Split(cookies, ":")[0], "=")[1]
	return cookie
}
