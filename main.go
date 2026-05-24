package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"golang.org/x/net/websocket"
)

const (
	width  = 60
	height = 40
	diff   = 0.2 // 拡散スピード（少し下げて滑らかに）
)

type Simulator struct {
	Current [][]float64
	Next    [][]float64
}

// ブラウザからのクリックイベントを受け取るための構造体
type ClickEvent struct {
	Type string `json:"type"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
}

func NewSimulator() *Simulator {
	curr := make([][]float64, height)
	next := make([][]float64, height)
	for i := range curr {
		curr[i] = make([]float64, width)
		next[i] = make([]float64, width)
	}
	// 初期圧力（中央にドカンと配置）
	curr[height/2][width/2] = 500.0
	return &Simulator{Current: curr, Next: next}
}

func (s *Simulator) Update() {
	// 拡散の計算（端の処理を考慮）
	for y := 1; y < height-1; y++ {
		for x := 1; x < width-1; x++ {
			p := s.Current[y][x]
			laplacian := s.Current[y-1][x] + s.Current[y+1][x] + s.Current[y][x-1] + s.Current[y][x+1] - 4*p
			s.Next[y][x] = p + diff*laplacian
		}
	}

	// データを次のステップに更新しつつ、全体を少しずつ減衰（自然に消えていくように）
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			s.Current[y][x] = s.Next[y][x] * 0.99
		}
	}
}

func main() {
	sim := NewSimulator()

	// 物理演算ループ（約60FPSで超高速更新）
	go func() {
		ticker := time.NewTicker(16 * time.Millisecond)
		for range ticker.C {
			sim.Update()
		}
	}()

	http.Handle("/ws", websocket.Handler(func(ws *websocket.Conn) {
		log.Println("ブラウザが接続しました")
		defer ws.Close()

		// ブラウザからのクリックを待ち受けるGoroutine
		go func() {
			for {
				var reply string
				if err := websocket.Message.Receive(ws, &reply); err != nil {
					break
				}
				var ev ClickEvent
				if err := json.Unmarshal([]byte(reply), &ev); err == nil && ev.Type == "click" {
					// クリックされた座標とその周囲の圧力をブースト
					for dy := -1; dy <= 1; dy++ {
						for dx := -1; dx <= 1; dx++ {
							ny, nx := ev.Y+dy, ev.X+dx
							if ny >= 0 && ny < height && nx >= 0 && nx < width {
								sim.Current[ny][nx] = 600.0
							}
						}
					}
				}
			}
		}()

		// ブラウザへのデータ送信ループ（約30FPS）
		ticker := time.NewTicker(33 * time.Millisecond)
		for range ticker.C {
			if err := websocket.JSON.Send(ws, sim.Current); err != nil {
				break
			}
		}
	}))

	http.Handle("/", http.FileServer(http.Dir(".")))

	fmt.Println("サーバー起動: http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}