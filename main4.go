/*
Date: 11/2024
Version: 1.2

Este es un juego de Tetris simple escrito en Go con el motor de juegos Ebiten.
El juego tiene un sistema de niveles, música de fondo y efectos de sonido, todo compuesto por Feri de FECORO,
lo mismo el pequeño arte gráfico.
El juego también tiene un sistema de highscores y una pantalla de inicio muy simple.

Para jugar, simplemente mueva las piezas con las teclas de flecha, rote con la tecla "Z", acelere la caída con
la tecla de flecha hacia abajo y pause con la tecla "P".

Para iniciar el juego, presione la barra espaciadora en la pantalla de inicio.

|--------------------------------------------------------------------------------------------------------------------|

This is a simple Tetris game written in Go with the Ebiten game engine.
The game has a level system, background music and sound effects, all composed by Feri from FECORO,
the same with the graphic art.
The game also has a highscores system and a very simple start screen.

To play, simply move the pieces with the arrow keys, rotate with the "Z" key, speed up the fall with the down arrow key,
and pause with the "P" key.

To start the game, press the space bar on the start screen.

This game was programmed in English and Spanish, and the comments are in Spanish, just because is my native tongue.
But as you can see, the game is very simple and easy to understand.

By Feri. (Rengo, Chile)
*/

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

// ....Constantes del juego....
const (
	PantallaWidth         = 800
	PantallaHeight        = 600
	GridWidth             = 10
	GridHeight            = 17
	TamañoCell            = 30
	SizeDelBlock          = 30
	VelocidadInicial      = 60
	ProbabiliSpecialPiece = 0.2
	LevelTimeLimitSeconds = 122 // 2 minutos por nivel

	//.... Estados del juego....
	EstadoCompany = iota //Estado primero
	EstadoStart
	EstadoPlayerName
	EstadoPlayMenu
	EstadoReglas
	EstadoHistoria
	EstadoMenu
	EstadoGame
	EstadoPause
	EstadoGameOver
	EstadoHighScores

	//.... Configuración de audio ....
	SampleRate      = 44100
	AudioBufferSize = 1024
)

// .... Estructura de datos para los highscores ....
type HighScore struct {
	Name  string
	Score int
	Level int
	Date  string
}

// .... Struct principal del juego ....
type Game struct {
	Estado          int
	grid            [GridHeight][GridWidth]int
	fallingX        int
	fallingY        int
	fallingCol      int
	fallingSpecial  bool
	fallingRotation int
	score           int
	level           int
	speed           int
	framesCounter   int
	message         string
	timer           int
	timeLimit       int
	musicWasPlaying bool
	inputText       string //inputnombre
	maxInputLength  int
	nextPieces      [3]int  //almacena 3 piezas spawneadas
	nextSpecial     [3]bool //almacena si las siguientes piezas son especiales
	//.... Pal movimiento de las piezas ....
	moveDelayCounter int
	moveDelay        int
	initialMoveDelay int
	keyHeldFrames    int
	lastMoveDir      int
	//.... Campos para la carga de recursos ....
	blockImage        *ebiten.Image
	specialMarks      map[string]*ebiten.Image
	highScores        []HighScore
	playerName        string
	inputActive       bool
	audioContext      *audio.Context
	sounds            map[string]*audio.Player
	bgm               *audio.Player
	bgm2              *audio.Player
	bgms              []*audio.Player //Array para almacenar las 5 canciones
	currentBgm        int             //Para rastrear la canción actual y así evitar reiniciarla
	font              font.Face
	retroFont         font.Face
	gameFont          font.Face
	storyFont         font.Face
	lastTimerUpdate   time.Time
	backgroundImage   *ebiten.Image //Mi imagen de fondo
	background2Image  *ebiten.Image //Mi imagen de juego
	companyImage      *ebiten.Image //Mi imagen de compañía
	iconimage         *ebiten.Image //Mi imagen de icono
	particles         []Particle
	lastParticleSpawn time.Time
	playMenuOption    int
}

// ..................................................................

func (g *Game) drawCompanyLogo(screen *ebiten.Image) {
	//Cargar imagen de fondo por unos segundos una unica vez
	if g.companyImage == nil {
		file, err := os.Open("componentes/company.png")
		if err != nil {
			log.Fatal(err)
		}
		img3, err := png.Decode(file)
		if err != nil {
			log.Fatal(err)
		}
		file.Close()

		g.companyImage = ebiten.NewImageFromImage(img3)
	}

	//Dibuja la imagen de fondo en la pantalla por 1 segundo:
	screen.DrawImage(g.companyImage, nil)

	//Espera 2 segundos y pasa al siguiente estado con una animación de fadeout
	if time.Since(g.lastTimerUpdate) > time.Second*1 {
		//fadeout
		if g.companyImage != nil {
			op := &ebiten.DrawImageOptions{}
			op.ColorM.Scale(1, 1, 1, 0.5)
			screen.DrawImage(g.companyImage, op)
		}

		//Pasa al siguiente estado
		g.Estado = EstadoStart
		g.lastTimerUpdate = time.Now()

	}
}

// .... Lógica de presentación y prejugo
// .... Estructura para una partícula individual ....
type Particle struct {
	x, y     float64
	speedX   float64
	speedY   float64
	size     float64
	lifetime float64
	alpha    float64
}

// .... Función para crear una nueva partícula ....
func newParticle() Particle {
	return Particle{
		x:        float64(rand.Intn(PantallaWidth)),
		y:        float64(rand.Intn(PantallaHeight)),
		speedX:   (rand.Float64() - 0.5) * 2,
		speedY:   (rand.Float64() - 0.5) * 2,
		size:     rand.Float64()*2 + 1,
		lifetime: 1.0,
		alpha:    1.0,
	}
}

// .... Función para actualizar las partículas ....
func (g *Game) updateParticles() {
	//Crea nuevas partículas periódicamente
	if time.Since(g.lastParticleSpawn) > time.Millisecond*50 {
		if len(g.particles) < 100 { //Límite máximo de partículas
			g.particles = append(g.particles, newParticle())
		}
		g.lastParticleSpawn = time.Now()
	}

	//Actualiza partículas existentes
	var activeParticles []Particle
	for _, p := range g.particles {
		p.x += p.speedX
		p.y += p.speedY
		p.lifetime -= 0.01
		p.alpha = p.lifetime

		//Mantene solo las partículas vivas
		if p.lifetime > 0 {
			//Hace que las partículas reboten en los bordes
			if p.x < 0 || p.x > float64(PantallaWidth) {
				p.speedX *= -1
			}
			if p.y < 0 || p.y > float64(PantallaHeight) {
				p.speedY *= -1
			}
			activeParticles = append(activeParticles, p)
		}
	}
	g.particles = activeParticles
}

// .... Función modificada drawStartScreen y que coloca las partículas ....
func (g *Game) drawStartScreen(screen *ebiten.Image) {
	//Cargar imagen de fondo
	if g.backgroundImage == nil {
		file, err := os.Open("componentes/background.png")
		if err != nil {
			log.Fatal(err)
		}
		img, err := png.Decode(file)
		if err != nil {
			log.Fatal(err)
		}
		file.Close()
		g.backgroundImage = ebiten.NewImageFromImage(img)
	}

	//Dibuja la imagen de fondo en la pantalla
	screen.DrawImage(g.backgroundImage, nil)

	//Actualiza y hace draw a las partículas
	g.updateParticles()

	//Crea imagen temporal para las partículas
	particleImg := ebiten.NewImage(3, 3)
	particleImg.Fill(color.RGBA{255, 255, 255, 255})

	//Dibuja cada partícula
	for _, p := range g.particles {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(-1.5, -1.5) //Centra la partícula
		op.GeoM.Scale(p.size, p.size)
		op.GeoM.Translate(p.x, p.y)

		//Configura su color y transparencia
		op.ColorM.Scale(1, 1, 1, p.alpha*0.3) //Ajusta el 0.3 para cambiar la opacidad general

		screen.DrawImage(particleImg, op)
	}

	pressStart := "PRESIONA ESPACIO PARA COMENZAR"
	if time.Now().UnixNano()/400000000%2 == 0 {
		text.Draw(screen, pressStart, g.retroFont,
			PantallaWidth/2-len(pressStart)*6,
			PantallaHeight*2/3,
			color.RGBA{255, 255, 255, 255})
	}

	//Copyright y créditos
	credits := "Copyright 2024 - FECORO"
	text.Draw(screen, credits, g.retroFont,
		PantallaWidth/2-len(credits)*6,
		PantallaHeight-50,
		color.RGBA{255, 255, 255, 255})

	//BGM2
	if g.bgm2 == nil {
		file, err := os.Open("componentes/music/bgmain.ogg")
		if err != nil {
			log.Fatal(err)
		}

		d, err := vorbis.Decode(g.audioContext, file)
		if err != nil {
			file.Close()
			log.Fatal(err)
		}

		g.bgm2, err = audio.NewPlayer(g.audioContext, d)
		if err != nil {
			file.Close()
			log.Fatal(err)
		}
	}

	//Reproducir BGM2
	if !g.bgm2.IsPlaying() {
		g.bgm2.Rewind()
		g.bgm2.Play()
	}
}

//..................................................................

// .... Update del start screen ....
func (g *Game) updateStartScreen() error {
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.Estado = EstadoPlayerName //Cambiado de EstadoPlayMenu a EstadoPlayerName
		g.inputText = ""            //Tira el texto vacío para input
		g.maxInputLength = 12       //Esto establece el límite de caracteres
		g.playSound("select")
	}

	//escape = salir
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		os.Exit(0)
	}
	return nil
}

//..................................................................

func (g *Game) updatePlayerName() error {
	//Manejamos el input de texto
	for _, char := range ebiten.InputChars() {
		if len(g.inputText) < g.maxInputLength {
			g.inputText += string(char)
		}
	}

	//Permitimos borrar caracteres
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(g.inputText) > 0 {
		g.inputText = g.inputText[:len(g.inputText)-1]
	}

	//Se confirma el nombre cuando presiona Espacio
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && len(g.inputText) > 0 {
		g.playerName = g.inputText
		g.Estado = EstadoPlayMenu
		g.playSound("select")
	}

	return nil
}

func (g *Game) drawPlayerName(screen *ebiten.Image) {
	// Dibuja el fondo
	bg := ebiten.NewImage(PantallaWidth, PantallaHeight)
	bg.Fill(color.RGBA{3, 5, 22, 255})
	screen.DrawImage(bg, nil)

	// Dibuja el título
	titleText := "INGRESA TU NOMBRE (MAX. 12 CARACTERES)"
	text.Draw(screen, titleText, g.retroFont,
		PantallaWidth/2-len(titleText)*6,
		PantallaHeight/3,
		color.White)

	// Dibuja el input actual
	inputText := g.inputText + "_"
	text.Draw(screen, inputText, g.retroFont,
		PantallaWidth/2-len(inputText)*6,
		PantallaHeight/2,
		color.RGBA{200, 200, 200, 255})

	// Dibuja las instrucciones
	instructions := "Presiona ENTER para confirmar"
	text.Draw(screen, instructions, g.retroFont,
		PantallaWidth/2-len(instructions)*6,
		PantallaHeight/2+70,
		color.RGBA{150, 150, 150, 255})
}

//..................................................................
//..................................................................

// .... Inicialización de juego nuevo, o sea un reset ....
func NewGame() *Game {
	g := &Game{
		Estado:           EstadoCompany,
		speed:            VelocidadInicial,
		level:            1,
		fallingRotation:  0,
		timeLimit:        LevelTimeLimitSeconds,
		specialMarks:     make(map[string]*ebiten.Image),
		sounds:           make(map[string]*audio.Player),
		lastTimerUpdate:  time.Now(),
		moveDelay:        4,
		initialMoveDelay: 10,
		lastMoveDir:      0,
	}

	g.loadResources()
	g.loadHighScores()
	g.initAudio()
	return g
}

// .... Carga de recursos ....
func (g *Game) loadResources() {
	//Cargar imagen base para los bloques
	blockFile, err := os.Open("componentes/block6.png")
	if err != nil {
		log.Fatal(err)
	}
	defer blockFile.Close()

	blockImg, err := png.Decode(blockFile)
	if err != nil {
		log.Fatal(err)
	}
	g.blockImage = ebiten.NewImageFromImage(blockImg)

	//Cargar marcas especiales
	specialShapes := []string{"star", "circle", "triangle"}
	for _, shape := range specialShapes {
		file, err := os.Open(fmt.Sprintf("componentes/%s.png", shape))
		if err != nil {
			log.Fatal(err)
		}
		img, err := png.Decode(file)
		if err != nil {
			log.Fatal(err)
		}
		file.Close()
		g.specialMarks[shape] = ebiten.NewImageFromImage(img)
	}

	//Cargar las fonts o letra
	fontData, err := ioutil.ReadFile("componentes/Pixel.ttf")
	if err != nil {
		log.Fatal(err)
	}

	tt, err := opentype.Parse(fontData)
	if err != nil {
		log.Fatal(err)
	}

	g.retroFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    24,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}

	//Cargar fuente de juego (gamefont):
	fontData2, err := ioutil.ReadFile("componentes/Gameplay.ttf")
	if err != nil {
		log.Fatal(err)
	}

	tt2, err := opentype.Parse(fontData2)
	if err != nil {
		log.Fatal(err)
	}

	g.gameFont, err = opentype.NewFace(tt2, &opentype.FaceOptions{
		Size:    32,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}

	g.storyFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    16,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}
}

// .... Dib
// Inicialización del audio
// .... Función initAudio que se encarga de cargar los archivos de audio y crear los reproductores correspondientes ....
var globalAudioContext *audio.Context

func (g *Game) initAudio() error {
	g.audioContext = globalAudioContext
	g.bgms = make([]*audio.Player, 6)
	g.sounds = make(map[string]*audio.Player)

	// Cargar BGMs .ogg
	for i := 0; i < 6; i++ {
		filePath := fmt.Sprintf("componentes/music/bgm%d.ogg", i+1) //Formato orbis de audio
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("error al abrir el archivo %s: %w", filePath, err)
		}

		d, err := vorbis.Decode(g.audioContext, file)
		if err != nil {
			file.Close() //Cierra el archivo en caso de error
			return fmt.Errorf("error al decodificar el archivo %s: %w", filePath, err)
		}

		player, err := audio.NewPlayer(g.audioContext, d)
		if err != nil {
			file.Close() //Cierra el archivo en caso de error
			return fmt.Errorf("error al crear el reproductor para %s: %w", filePath, err)
		}
		g.bgms[i] = player
	}

	//Cargar efectos de sonido
	soundFiles := []string{"match", "levelup", "gameover", "special", "lock", "select"}
	for _, name := range soundFiles {
		filePath := fmt.Sprintf("componentes/sounds/%s.wav", name)
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("error al abrir el archivo %s: %w", filePath, err)
		}

		d, err := wav.Decode(g.audioContext, file)
		if err != nil {
			file.Close() //Cierra el archivo en caso de error
			return fmt.Errorf("error al decodificar el archivo %s: %w", filePath, err)
		}

		player, err := audio.NewPlayer(g.audioContext, d)
		if err != nil {
			file.Close() //Cierra el archivo en caso de error
			return fmt.Errorf("error al crear el reproductor para %s: %w", filePath, err)
		}
		g.sounds[name] = player
	}

	return nil
}

// .... Estructura personalizada para implementar ReadSeekCloser .... (Por si acaso)
type customReader struct {
	*bytes.Reader
}

// .... Implementación del método Close para satisfacer la interfaz ReadSeekCloser ....
func (r *customReader) Close() error {
	return nil
}

// .... Manejo de highscores ....
func (g *Game) loadHighScores() {
	data, err := ioutil.ReadFile("puntajes.json")
	if err == nil {
		json.Unmarshal(data, &g.highScores)
	}
	sort.Slice(g.highScores, func(i, j int) bool {
		return g.highScores[i].Score > g.highScores[j].Score
	})
}

func (g *Game) saveHighScore() {
	now := time.Now()
	newScore := HighScore{
		Name:  g.playerName,
		Score: g.score,
		Level: g.level,
		Date:  now.Format("2006-01-02 15:04:05"),
	}

	g.highScores = append(g.highScores, newScore)
	sort.Slice(g.highScores, func(i, j int) bool {
		return g.highScores[i].Score > g.highScores[j].Score
	})

	if len(g.highScores) > 10 {
		g.highScores = g.highScores[:10]
	}

	data, _ := json.Marshal(g.highScores)
	ioutil.WriteFile("puntajes.json", data, 0644)
}

// .... Actualización del juego, aquí se manejan los estados y las acciones del juego ....
func (g *Game) Update() error {
	switch g.Estado {
	case EstadoStart:
		return g.updateStartScreen()
	case EstadoPlayerName:
		return g.updatePlayerName()
	case EstadoMenu:
		//Toca canción bgmain:
		return g.updateMenu()
	case EstadoGame:
		//parar musica bgm2 si está sonando
		if g.bgm2 != nil && g.bgm2.IsPlaying() {
			g.bgm2.Pause()
		}
		return g.updateGame()
	case EstadoPause:
		return g.updatePause()
	case EstadoGameOver:
		return g.updateGameOver()
	case EstadoHighScores:
		return g.updateHighScores()
	}
	return nil
}

func (g *Game) updateMenu() error {
	//Espacio y Enter para iniciar el juego
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		g.Estado = EstadoGame
		g.startGame()
		g.playSound("select")
		g.playBGM()
	} else if inpututil.IsKeyJustPressed(ebiten.KeyH) {
		g.Estado = EstadoHighScores
		g.playSound("select")
		//KeyV o KeyESC:
	} else if inpututil.IsKeyJustPressed(ebiten.KeyLeft) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.Estado = EstadoPlayMenu
		g.playSound("select")
	} else if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		//salir del juego
		os.Exit(0)
	}
	return nil
}

//..................................................................

// .... Función de menu de selección de juego ....
func (g *Game) drawPlayMenu(screen *ebiten.Image) {
	//Se colorea el fondo de pantalla con el mismo que menú
	screen.Fill(color.RGBA{29, 29, 41, 255})

	//Aquí se dibuja la pantalla de selección de juego, con las opciones de juego (y una flecha señalando la opción seleccionada)
	text.Draw(screen, "SELECCIONA CON LA FLECHA DERECHA", g.retroFont, 200, 100, color.White)

	//Opciones de menú
	options := []string{"JUGAR", "REGLAS", "PUNTAJES", "HISTORIA", "ENTRADA", "SALIR"}
	for i, option := range options {
		text.Draw(screen, option, g.retroFont, 200, 200+i*50, color.White)
	}

	//Flecha de selección
	text.Draw(screen, ">", g.retroFont, 150, 200+g.playMenuOption*50, color.White)

	//Partículas
	g.updateParticles()

	//Crea imagen temporal para las partículas
	particleImg := ebiten.NewImage(3, 3)
	particleImg.Fill(color.RGBA{255, 255, 255, 255})

	//Dibuja cada partícula
	for _, p := range g.particles {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(-1.5, -1.5) //Centra la partícula
		op.GeoM.Scale(p.size, p.size)
		op.GeoM.Translate(p.x, p.y)

		//Configura su color y transparencia
		op.ColorM.Scale(1, 1, 1, p.alpha*0.3) //Ajusta el 0.3 para cambiar la opacidad general

		screen.DrawImage(particleImg, op)
	}
	g.updatePlayMenu()

}

// .... Update del menu de selección de juego ....
func (g *Game) updatePlayMenu() error { //debe mover la flecha de selección
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		g.playMenuOption++
		g.playSound("select")
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		g.playMenuOption--
		g.playSound("select")
	}

	if g.playMenuOption < 0 {
		g.playMenuOption = 5
	}

	if g.playMenuOption > 5 {
		g.playMenuOption = 0
	}

	//Escape para volver al menú principal
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		g.Estado = EstadoStart
		g.playSound("select")
	}

	//Selección de opción (pero necesita apretar enter)
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		switch g.playMenuOption {
		case 0:
			g.Estado = EstadoMenu
			g.playSound("select")
		case 1:
			g.Estado = EstadoReglas
			g.playSound("select")
		case 4:
			g.Estado = EstadoStart
			g.playSound("select")
		case 2:
			g.Estado = EstadoHighScores
			g.playSound("select")
		case 3:
			g.Estado = EstadoHistoria
			g.playSound("select")
		case 5:
			os.Exit(0)
		}
	}

	return nil
}

// .... Función de reglas del juego ....
func (g *Game) drawRules(screen *ebiten.Image) {
	//text.Draw(screen, "REGLAS DEL JUEGO", g.retroFont, 200, 100, color.White)
	//text.Draw(screen, "Mueve las piezas con las flechas", g.retroFont, 200, 150, color.White)
	//text.Draw(screen, "Rota con la tecla Z", g.retroFont, 200, 200, color.White)
	//text.Draw(screen, "Acelera la caída con la flecha abajo", g.retroFont, 200, 250, color.White)
	//text.Draw(screen, "Pausa con la tecla P", g.retroFont, 200, 300, color.White)
	//text.Draw(screen, "Presiona ESPACIO para volver al menú", g.retroFont, 200, 350, color.White)
	//text.Draw(screen, "", g.retroFont, 200, 350, color.White)

	//text.Draw(screen, "El objetivo es hacer líneas horizontales", g.retroFont, 200, 400, color.White)
	//text.Draw(screen, "para ganar puntos y subir de nivel", g.retroFont, 200, 450, color.White)
	//text.Draw(screen, "Cada nivel tiene un tiempo límite", g.retroFont, 200, 500, color.White)
	//text.Draw(screen, "Las piezas marcadas especiales entrega un bonus", g.retroFont, 200, 550, color.White)
	//text.Draw(screen, "si logras lockearlas.", g.retroFont, 200, 600, color.White)
	//text.Draw(screen, "La pieza multicolor es especial y da muchos puntos", g.retroFont, 200, 650, color.White)
	//text.Draw(screen, "Puedes ver los highscores en el menú principal", g.retroFont, 200, 700, color.White)
	//text.Draw(screen, "Vuelve al menú con ESC", g.retroFont, 200, 750, color.White)

	//Las reglas escritas de manera legible y con un scroll en pantalla
	rules := []string{
		"REGLAS DEL JUEGO",
		"Moverás las piezas con las flechas de tu teclado.",
		"Rota con la tecla Z o la flecha arriba.",
		"Acelera la caída con las teclas flecha abajo o X.",
		"Tu objetivo es hacer líneas horizontales,",
		"para ganar puntos y aguantar el tiempo.",
		"Las piezas marcadas te entregan un pequeño bonus,",
		"si logras lockearlas.",
		"La pieza multicolor es especial y cambia de forma,",
		"hacer una línea con ella da muchos puntos.",
		"Al avanzar de nivel, la velocidad aumenta.",
	}

	//Mensaje 'Vuelve al menú con ESC o derecha':
	text.Draw(screen, "Vuelve al menú con ESC o ←", g.retroFont, 100, 500, (color.RGBA{150, 150, 150, 255}))

	for i, rule := range rules {
		text.Draw(screen, rule, g.retroFont, 100, 100+i*30, color.White)
	}

	//Partículas
	g.updateParticles()

	//Crea imagen temporal para las partículas
	particleImg := ebiten.NewImage(3, 3)
	particleImg.Fill(color.RGBA{255, 255, 255, 255})

	//Dibuja cada partícula
	for _, p := range g.particles {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(-1.5, -1.5) //Centra la partícula
		op.GeoM.Scale(p.size, p.size)
		op.GeoM.Translate(p.x, p.y)

		//Configura su color y transparencia
		op.ColorM.Scale(1, 1, 1, p.alpha*0.3) //Ajusta el 0.3 para cambiar la opacidad general
		screen.DrawImage(particleImg, op)
	}

	g.updateRules()
}

// .... Update de las reglas del juego ....
func (g *Game) updateRules() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		g.Estado = EstadoPlayMenu
		g.playSound("select")
	}
	return nil
}

// .... Función de historia del juego ....
func (g *Game) drawHistoria(screen *ebiten.Image) {
	text.Draw(screen, "LORE", g.retroFont, 100, 100, color.White)

	//Multilinea:
	historia := []string{

		"Año 2437, después de la era de las máquinas:",
		"La humanidad intentó conquistar las estrellas, pero no estaba sola.",
		"Esta fue atacada por diversos seres extraplanetarios desde el año 2430,",
		"esto causó la destrucción de la Tierra, solo salvándose algunos en naves X97.",
		"",
		"Como capitán de la nave Fetris, tu misión es proteger la última esperanza",
		"de la humanidad, la tripulación que llevas de viaje hacia un nuevo hogar.",
		"",
		"Con los cubos mineros estelares que encuentres en tu camino, construye",
		"líneas de defensa sin vacíos, y asegura la supervivencia de tu tripulación.",
		"",
		"El destino de la humanidad está en tus manos. Piensa y sobrevivirás.",
	}

	for i, line := range historia {
		text.Draw(screen, line, g.storyFont, 100, 150+i*30, color.White)
	}

	//Mensaje 'Vuelve al menú con ESC':
	text.Draw(screen, "Vuelve al menú con ESC o ←", g.retroFont, 100, 540, (color.RGBA{150, 150, 150, 255}))

	//Partículas
	g.updateParticles()

	//Crea imagen temporal para las partículas
	particleImg := ebiten.NewImage(3, 3)
	particleImg.Fill(color.RGBA{255, 255, 255, 255})

	//Dibuja cada partícula
	for _, p := range g.particles {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(-1.5, -1.5) //Centra la partícula
		op.GeoM.Scale(p.size, p.size)
		op.GeoM.Translate(p.x, p.y)

		//Configura su color y transparencia
		op.ColorM.Scale(1, 1, 1, p.alpha*0.3) //Ajusta el 0.3 para cambiar la opacidad general
		screen.DrawImage(particleImg, op)
	}

	g.updateHistoria()
}

// .... Update de la historia del juego ....
func (g *Game) updateHistoria() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		g.Estado = EstadoPlayMenu
		g.playSound("select")
	}
	return nil
}

// .... Función para iniciar un nuevo juego ....
func (g *Game) startGame() {
	g.grid = [GridHeight][GridWidth]int{}
	g.score = 0
	g.level = 1
	g.speed = VelocidadInicial
	g.timer = g.timeLimit
	g.currentBgm = 0
	g.bgms[g.currentBgm].Play()

	// Inicializa las piezas preview
	for i := 0; i < 3; i++ {
		g.nextPieces[i] = rand.Intn(11) + 1
		g.nextSpecial[i] = rand.Float64() < ProbabiliSpecialPiece
	}

	g.spawnPiece()
}

// La función que se utiliza para el cambio de BGM
func (g *Game) changeBgmForLevel() {
	// Detener la música actual
	if g.bgms[g.currentBgm] != nil && g.bgms[g.currentBgm].IsPlaying() {
		g.bgms[g.currentBgm].Pause()
	}

	//Calcula qué BGM debe sonar (de los 7 o los que se definan)
	newBgm := (g.level - 1) % 8

	//Si es diferente BGM, reiniciar y reproducir
	if newBgm != g.currentBgm {
		if g.bgms[g.currentBgm] != nil {
			g.bgms[g.currentBgm].Rewind()
		}
		g.currentBgm = newBgm
	}

	if g.bgms[g.currentBgm] != nil {
		g.bgms[g.currentBgm].Play()
	}
}

// ..................................................................
// Pa saber si está cayendo una pieza
func (g *Game) isFallingPiece() bool {
	return g.fallingY >= 0
}

func (g *Game) handleSmoothMovement() {
	//Obvio, si no hay pieza cayendo, no hacemos nada
	if !g.isFallingPiece() {
		return
	}

	//Detecta entrada del teclado
	moveDir := 0
	if ebiten.IsKeyPressed(ebiten.KeyLeft) {
		moveDir = -1
	} else if ebiten.IsKeyPressed(ebiten.KeyRight) {
		moveDir = 1
	}

	//Si no hay movimiento, resetear contadores
	if moveDir == 0 {
		g.moveDelayCounter = 0
		g.keyHeldFrames = 0
		g.lastMoveDir = 0
		return
	}

	//Si es una nueva dirección, resetear contadores
	if moveDir != g.lastMoveDir {
		g.moveDelayCounter = 0
		g.keyHeldFrames = 0
		g.lastMoveDir = moveDir
	}

	g.keyHeldFrames++

	//Mover inmediatamente en el primer frame
	if g.keyHeldFrames == 1 {
		if g.canMove(moveDir, 0) {
			g.fallingX += moveDir
		}
		return
	}

	//Esperar el delay inicial
	if g.keyHeldFrames <= g.initialMoveDelay {
		return
	}

	//Incrementar el contador de delay
	g.moveDelayCounter++

	//Si alcanzamos el delay deseado, mover la pieza
	if g.moveDelayCounter >= g.moveDelay {
		if g.canMove(moveDir, 0) {
			g.fallingX += moveDir
		}
		g.moveDelayCounter = 0
	}
}

//..................................................................
//..................................................................

// .... Función para el manejo de la lógica del juego ....
func (g *Game) updateGame() error {
	now := time.Now()

	//Actualizar timer cada segundo, independiente de los frames y teclas
	if now.Sub(g.lastTimerUpdate) >= time.Second {
		g.timer--
		g.lastTimerUpdate = now

		if g.timer <= 0 {
			if !g.checkLevelComplete() {
				g.gameOver()
			} else {
				g.nextLevel()
			}
		}
	}

	//Sistema de movimiento horizontal
	moveDir := 0
	if ebiten.IsKeyPressed(ebiten.KeyLeft) {
		moveDir = -1
	} else if ebiten.IsKeyPressed(ebiten.KeyRight) {
		moveDir = 1
	}

	if moveDir != 0 {
		if moveDir != g.lastMoveDir {
			// Primer movimiento inmediato al presionar la tecla
			if g.canMove(moveDir, 0) {
				g.fallingX += moveDir
			}
			g.moveDelayCounter = 0
			g.keyHeldFrames = 1
			g.lastMoveDir = moveDir
		} else {
			g.keyHeldFrames++
			if g.keyHeldFrames > g.initialMoveDelay {
				g.moveDelayCounter++
				if g.moveDelayCounter >= g.moveDelay {
					if g.canMove(moveDir, 0) {
						g.fallingX += moveDir
					}
					g.moveDelayCounter = 0
				}
			}
		}
	} else {
		g.moveDelayCounter = 0
		g.keyHeldFrames = 0
		g.lastMoveDir = 0
	}

	//Caída rápida
	if ebiten.IsKeyPressed(ebiten.KeyDown) {
		g.framesCounter += g.speed / 4
	}

	//Caída instantánea (X o Espacio)
	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		for g.canMove(0, 1) {
			g.fallingY++
		}
		g.lockPiece()
		g.spawnPiece()
	}

	//Actualización de la caída de la pieza
	g.framesCounter++
	if g.framesCounter >= g.speed {
		g.framesCounter = 0
		if g.canMove(0, 1) {
			g.fallingY++
		} else {
			g.lockPiece()
			g.spawnPiece()
		}
	}

	//Pausita
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		g.Estado = EstadoPause
		g.bgms[g.currentBgm].Pause()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.Estado = EstadoMenu
		//parar la música si está sonando y reiniciar, o sea un STOP:
		if g.bgms[g.currentBgm] != nil && g.bgms[g.currentBgm].IsPlaying() {
			g.bgms[g.currentBgm].Pause()
			g.bgms[g.currentBgm].Rewind()
		}
	}

	return nil
}

func (g *Game) canMove(dx, dy int) bool {
	//Se definen las formas de los tetrominos con sus 4 rotaciones
	tetrominos := map[int][][]struct{ x, y int }{
		1: { // I
			{{0, 0}, {1, 0}, {2, 0}, {3, 0}},
			{{1, -1}, {1, 0}, {1, 1}, {1, 2}},
			{{0, 1}, {1, 1}, {2, 1}, {3, 1}},
			{{2, -1}, {2, 0}, {2, 1}, {2, 2}},
		},
		2: { // O - No rota
			{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
			{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
			{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
			{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
		},
		3: { // T
			{{1, 0}, {0, 1}, {1, 1}, {2, 1}},
			{{1, 0}, {1, 1}, {2, 1}, {1, 2}},
			{{0, 1}, {1, 1}, {2, 1}, {1, 2}},
			{{1, 0}, {0, 1}, {1, 1}, {1, 2}},
		},
		4: { // L
			{{0, 0}, {0, 1}, {0, 2}, {1, 2}},
			{{0, 0}, {1, 0}, {2, 0}, {0, 1}},
			{{0, 0}, {1, 0}, {1, 1}, {1, 2}},
			{{2, 0}, {0, 1}, {1, 1}, {2, 1}},
		},
		5: { // J
			{{1, 0}, {1, 1}, {0, 2}, {1, 2}},
			{{0, 0}, {0, 1}, {1, 1}, {2, 1}},
			{{0, 0}, {1, 0}, {0, 1}, {0, 2}},
			{{0, 0}, {1, 0}, {2, 0}, {2, 1}},
		},
		6: { // Z
			{{0, 0}, {1, 0}, {1, 1}, {2, 1}},
			{{2, 0}, {1, 1}, {2, 1}, {1, 2}},
			{{0, 1}, {1, 1}, {1, 2}, {2, 2}},
			{{1, 0}, {0, 1}, {1, 1}, {0, 2}},
		},
		7: { // S
			{{1, 0}, {2, 0}, {0, 1}, {1, 1}},
			{{1, 0}, {1, 1}, {2, 1}, {2, 2}},
			{{1, 1}, {2, 1}, {0, 2}, {1, 2}},
			{{0, 0}, {0, 1}, {1, 1}, {1, 2}},
		},
		8: { // U
			{{1, 0}, {0, 0}, {0, 1}, {0, 2}, {1, 2}},
			{{0, 1}, {0, 0}, {1, 0}, {2, 0}, {2, 1}},
			{{0, 0}, {1, 0}, {1, 1}, {1, 2}, {0, 2}},
			{{0, 0}, {0, 1}, {1, 1}, {2, 1}, {2, 0}},
		},
		9: { // Pieza Especial 1
			{{2, 0}, {2, 1}, {1, 1}, {1, 2}, {0, 2}},
			{{1, 0}, {1, 1}, {1, 2}, {0, 1}, {2, 1}},
			{{0, 0}, {0, 1}, {1, 1}, {2, 1}, {2, 2}},
			{{2, 0}, {2, 1}, {1, 1}, {0, 1}, {0, 2}},
		},
		10: { // | grande
			{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}},
			{{2, 0}, {2, 1}, {2, 2}, {2, 3}, {2, 4}},
			{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}},
			{{2, 0}, {2, 1}, {2, 2}, {2, 3}, {2, 4}},
		},
		11: { // Otra
			{{0, 2}, {1, 2}, {2, 2}, {0, 1}, {0, 0}},
			{{0, 0}, {1, 0}, {2, 0}, {0, 1}, {0, 2}},
			{{0, 0}, {1, 0}, {2, 0}, {2, 1}, {2, 2}},
			{{0, 2}, {1, 2}, {2, 2}, {2, 1}, {2, 0}},
		},
	}

	//Obtenemos la forma del tetromino actual con su rotación correspondiente
	if blocks := tetrominos[g.fallingCol][g.fallingRotation]; len(blocks) > 0 {
		for _, block := range blocks {
			//Calcula nueva posición
			x := g.fallingX + block.x + dx
			y := g.fallingY + block.y + dy

			//Verificamos límites del grid (importante)
			if x < 0 || x >= GridWidth || y >= GridHeight {
				return false
			}

			//Aquí es verificar colisión con otras piezas (sin dividir la pieza)
			if y >= 0 && g.grid[y][x] != 0 {
				return false
			}
		}
		return true
	}

	return false
}

// .... Función para obtener la rotación de una pieza en el grid ....
func (g *Game) rotationFromGrid(y, x int) int {
	//Se definen nuevamente las formas de los tetrominos con sus 4 rotaciones (soy flojo)
	tetrominos := map[int][][]struct{ x, y int }{
		1: { // I
			{{0, 0}, {1, 0}, {2, 0}, {3, 0}},
			{{1, -1}, {1, 0}, {1, 1}, {1, 2}},
			{{0, 1}, {1, 1}, {2, 1}, {3, 1}},
			{{2, -1}, {2, 0}, {2, 1}, {2, 2}},
		},
		2: { // O - No rota
			{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
			{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
			{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
			{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
		},
		3: { // T
			{{1, 0}, {0, 1}, {1, 1}, {2, 1}},
			{{1, 0}, {1, 1}, {2, 1}, {1, 2}},
			{{0, 1}, {1, 1}, {2, 1}, {1, 2}},
			{{1, 0}, {0, 1}, {1, 1}, {1, 2}},
		},
		4: { // L
			{{0, 0}, {0, 1}, {0, 2}, {1, 2}},
			{{0, 0}, {1, 0}, {2, 0}, {0, 1}},
			{{0, 0}, {1, 0}, {1, 1}, {1, 2}},
			{{2, 0}, {0, 1}, {1, 1}, {2, 1}},
		},
		5: { // J
			{{1, 0}, {1, 1}, {0, 2}, {1, 2}},
			{{0, 0}, {0, 1}, {1, 1}, {2, 1}},
			{{0, 0}, {1, 0}, {0, 1}, {0, 2}},
			{{0, 0}, {1, 0}, {2, 0}, {2, 1}},
		},
		6: { // Z
			{{0, 0}, {1, 0}, {1, 1}, {2, 1}},
			{{2, 0}, {1, 1}, {2, 1}, {1, 2}},
			{{0, 1}, {1, 1}, {1, 2}, {2, 2}},
			{{1, 0}, {0, 1}, {1, 1}, {0, 2}},
		},
		7: { // S
			{{1, 0}, {2, 0}, {0, 1}, {1, 1}},
			{{1, 0}, {1, 1}, {2, 1}, {2, 2}},
			{{1, 1}, {2, 1}, {0, 2}, {1, 2}},
			{{0, 0}, {0, 1}, {1, 1}, {1, 2}},
		},
		8: { // U
			{{1, 0}, {0, 0}, {0, 1}, {0, 2}, {1, 2}},
			{{0, 1}, {0, 0}, {1, 0}, {2, 0}, {2, 1}},
			{{0, 0}, {1, 0}, {1, 1}, {1, 2}, {0, 2}},
			{{0, 0}, {0, 1}, {1, 1}, {2, 1}, {2, 0}},
		},
		9: { // Pieza Especial 1
			{{2, 0}, {2, 1}, {1, 1}, {1, 2}, {0, 2}},
			{{1, 0}, {1, 1}, {1, 2}, {0, 1}, {2, 1}},
			{{0, 0}, {0, 1}, {1, 1}, {2, 1}, {2, 2}},
			{{2, 0}, {2, 1}, {1, 1}, {0, 1}, {0, 2}},
		},
		10: { // | grande
			{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}},
			{{2, 0}, {2, 1}, {2, 2}, {2, 3}, {2, 4}},
			{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}},
			{{2, 0}, {2, 1}, {2, 2}, {2, 3}, {2, 4}},
		},
		11: { // Otra
			{{0, 2}, {1, 2}, {2, 2}, {0, 1}, {0, 0}},
			{{0, 0}, {1, 0}, {2, 0}, {0, 1}, {0, 2}},
			{{0, 0}, {1, 0}, {2, 0}, {2, 1}, {2, 2}},
			{{0, 2}, {1, 2}, {2, 2}, {2, 1}, {2, 0}},
		},
	}

	for i, blocks := range tetrominos[g.grid[y][x]] {
		for _, b := range blocks {
			if b.x == x-g.fallingX && b.y == y-g.fallingY {
				return i
			}
		}
	}
	return 0
}

func (g *Game) lockPiece() {
	//Idem a la función de rotaciónFromGrid
	tetrominos := map[int][][]struct{ x, y int }{
		1: { // I
			{{0, 0}, {1, 0}, {2, 0}, {3, 0}},
			{{1, -1}, {1, 0}, {1, 1}, {1, 2}},
			{{0, 1}, {1, 1}, {2, 1}, {3, 1}},
			{{2, -1}, {2, 0}, {2, 1}, {2, 2}},
		},
		2: { // O - No rota
			{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
			{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
			{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
			{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
		},
		3: { // T
			{{1, 0}, {0, 1}, {1, 1}, {2, 1}},
			{{1, 0}, {1, 1}, {2, 1}, {1, 2}},
			{{0, 1}, {1, 1}, {2, 1}, {1, 2}},
			{{1, 0}, {0, 1}, {1, 1}, {1, 2}},
		},
		4: { // L
			{{0, 0}, {0, 1}, {0, 2}, {1, 2}},
			{{0, 0}, {1, 0}, {2, 0}, {0, 1}},
			{{0, 0}, {1, 0}, {1, 1}, {1, 2}},
			{{2, 0}, {0, 1}, {1, 1}, {2, 1}},
		},
		5: { // J
			{{1, 0}, {1, 1}, {0, 2}, {1, 2}},
			{{0, 0}, {0, 1}, {1, 1}, {2, 1}},
			{{0, 0}, {1, 0}, {0, 1}, {0, 2}},
			{{0, 0}, {1, 0}, {2, 0}, {2, 1}},
		},
		6: { // Z
			{{0, 0}, {1, 0}, {1, 1}, {2, 1}},
			{{2, 0}, {1, 1}, {2, 1}, {1, 2}},
			{{0, 1}, {1, 1}, {1, 2}, {2, 2}},
			{{1, 0}, {0, 1}, {1, 1}, {0, 2}},
		},
		7: { // S
			{{1, 0}, {2, 0}, {0, 1}, {1, 1}},
			{{1, 0}, {1, 1}, {2, 1}, {2, 2}},
			{{1, 1}, {2, 1}, {0, 2}, {1, 2}},
			{{0, 0}, {0, 1}, {1, 1}, {1, 2}},
		},
		8: { // U
			{{1, 0}, {0, 0}, {0, 1}, {0, 2}, {1, 2}},
			{{0, 1}, {0, 0}, {1, 0}, {2, 0}, {2, 1}},
			{{0, 0}, {1, 0}, {1, 1}, {1, 2}, {0, 2}},
			{{0, 0}, {0, 1}, {1, 1}, {2, 1}, {2, 0}},
		},
		9: { // Pieza Especial 1
			{{2, 0}, {2, 1}, {1, 1}, {1, 2}, {0, 2}},
			{{1, 0}, {1, 1}, {1, 2}, {0, 1}, {2, 1}},
			{{0, 0}, {0, 1}, {1, 1}, {2, 1}, {2, 2}},
			{{2, 0}, {2, 1}, {1, 1}, {0, 1}, {0, 2}},
		},
		10: { // | grande
			{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}},
			{{2, 0}, {2, 1}, {2, 2}, {2, 3}, {2, 4}},
			{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}},
			{{2, 0}, {2, 1}, {2, 2}, {2, 3}, {2, 4}},
		},
		11: { // Otra
			{{0, 2}, {1, 2}, {2, 2}, {0, 1}, {0, 0}},
			{{0, 0}, {1, 0}, {2, 0}, {0, 1}, {0, 2}},
			{{0, 0}, {1, 0}, {2, 0}, {2, 1}, {2, 2}},
			{{0, 2}, {1, 2}, {2, 2}, {2, 1}, {2, 0}},
		},
	}

	//Obtenemos la forma actual según la rotación
	currentShape := tetrominos[g.fallingCol][g.fallingRotation]

	//Verificamos si toda la pieza puede ser colocada
	canLock := true
	for _, block := range currentShape {
		newX := g.fallingX + block.x
		newY := g.fallingY + block.y

		//Importante: límites y colisiones
		if newX < 0 || newX >= GridWidth || newY >= GridHeight {
			canLock = false
			break
		}
		if newY >= 0 && g.grid[newY][newX] != 0 {
			canLock = false
			break
		}
	}

	//Si no se puede colocar, ajustar la posición Y hacia arriba
	if !canLock {
		g.fallingY--
	}

	//Colocamos la pieza completa en su posición final y mantener color y rotación
	for _, block := range currentShape {

		newX := g.fallingX + block.x
		newY := g.fallingY + block.y

		if newY >= 0 {
			g.grid[newY][newX] = g.fallingCol
		}
	}

	//Verificar si la pieza es especial y tocar sonido combo al lockear
	if g.fallingSpecial {
		g.playSound("special")
	}

	//Puntos por lockear pieza especial
	if g.fallingSpecial {
		g.score += 100
	}

	//puntos por lockear pieza normal
	g.score += 10

	//Reproducir sonido de bloqueo
	g.playSound("lock")
	g.checkAndClearMatches()
}

// .... Función para chequear si un nivel está completo ....
func (g *Game) checkLevelComplete() bool {
	//Verificar si hay menos del 99% de celdas ocupadas
	occupied := 0
	total := GridWidth * GridHeight

	for y := 0; y < GridHeight; y++ {
		for x := 0; x < GridWidth; x++ {
			if g.grid[y][x] != 0 {
				occupied++
			}
		}
	}

	return float64(occupied)/float64(total) < 0.99
}

func (g *Game) nextLevel() {
	g.level++
	g.timer = g.timeLimit
	g.speed = max(5, VelocidadInicial-g.level*6)
	g.playSound("levelup")
	// Mostrar mensaje de nivel y luego borrarlo
	g.message = fmt.Sprintf("NIVEL %d", g.level)
	go func() {
		time.Sleep(2 * time.Second)
		g.message = ""
	}()

	g.changeBgmForLevel()
}

// .... Función de game over ....
func (g *Game) gameOver() {
	g.Estado = EstadoGameOver
	g.playSound("gameover")
	// Deja de tocar la música
	if g.bgms[g.currentBgm] != nil && g.bgms[g.currentBgm].IsPlaying() {
		g.bgms[g.currentBgm].Pause()
		g.bgms[g.currentBgm].Rewind()
	}
	g.saveHighScore()
}

// Verificamos si una línea (de las que se chequean) es especial
func (g *Game) checkSpecialLine(y int) bool {
	for x := 0; x < GridWidth; x++ {
		if g.grid[y][x] == 0 {
			return false
		}
	}
	return true
}

// ....Función para chequear y limpiar líneas completas ....
func (g *Game) checkAndClearMatches() {
	//Reglas de un tetris normal: 1 línea = 100 puntos, 2 líneas = 300 puntos, 3 líneas = 500 puntos, 4 líneas = 800 puntos
	//Si se eliminan más de 4 líneas a la vez, se obtiene un bonus de 1200 puntos
	//Si se elimina una línea especial, se obtiene un bonus de 200 puntos
	//Si se elimina una línea especial y una normal, se obtiene un bonus de 400 puntos

	// Verificar si hay líneas completas
	lines := 0
	specialLines := 0
	for y := 0; y < GridHeight; y++ {
		full := true
		for x := 0; x < GridWidth; x++ {
			if g.grid[y][x] == 0 {
				full = false
				break
			}
		}

		if full {
			lines++
			if g.checkSpecialLine(y) {
				specialLines++
			}
			// Eliminar la línea
			for y2 := y; y2 > 0; y2-- {
				for x := 0; x < GridWidth; x++ {
					g.grid[y2][x] = g.grid[y2-1][x]
				}
			}
		}

	}

	//Calcular puntaje
	points := 0
	if lines > 0 {
		switch lines {
		case 1:
			points = 100
		case 2:
			points = 300
		case 3:
			points = 500
		case 4:
			points = 800
		default:
			points = 1200
		}

		//Bonuses por líneas especiales
		if specialLines > 0 {
			points += specialLines * 200
		}

		//Bonus por líneas especiales y normales
		if specialLines > 0 && lines > 0 {
			points += specialLines * 200

		}

		//Aplicar puntaje
		g.score += points
		g.playSound("match")
	}
}

// .... Función de update para el estado de pausa, aquí se manejan las acciones ....
func (g *Game) updatePause() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		g.Estado = EstadoGame
		if g.bgms[g.currentBgm] != nil {
			g.bgms[g.currentBgm].Play()
		}
	} else if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.Estado = EstadoMenu
	}
	return nil
}

func (g *Game) updateGameOver() error {
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.Estado = EstadoMenu
		//pausar sonido de gameover
		if g.sounds["gameover"] != nil && g.sounds["gameover"].IsPlaying() {
			g.sounds["gameover"].Pause()
			g.sounds["gameover"].Rewind()
		}
		g.startGame()

		//Detener la música si está sonando y reiniciar, o sea un STOP:
		if g.bgms[g.currentBgm] != nil && g.bgms[g.currentBgm].IsPlaying() {
			g.bgms[g.currentBgm].Pause()
			g.bgms[g.currentBgm].Rewind()
		}

	}

	if inpututil.IsKeyJustPressed(ebiten.KeyH) {
		g.Estado = EstadoHighScores
		//pausar sonido de gameover
		if g.sounds["gameover"] != nil && g.sounds["gameover"].IsPlaying() {
			g.sounds["gameover"].Pause()
			g.sounds["gameover"].Rewind()
		}

	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.Estado = EstadoMenu
		//pausar sonido de gameover
		if g.sounds["gameover"] != nil && g.sounds["gameover"].IsPlaying() {
			g.sounds["gameover"].Pause()
			g.sounds["gameover"].Rewind()
		}
	}

	return nil
}

func (g *Game) updateHighScores() error {
	//Solo si se preta escape una vez, para no saltar 2 veces de pantalla
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		g.Estado = EstadoPlayMenu
		//sonido select
		g.playSound("select")
	}

	return nil
}

func (g *Game) spawnPiece() {
	g.fallingX = GridWidth / 2
	g.fallingY = 0

	//Usa la primera pieza del preview
	g.fallingCol = g.nextPieces[0]
	g.fallingSpecial = g.nextSpecial[0]

	//Esto hace que se muevan todas las piezas una posición
	for i := 0; i < 2; i++ {
		g.nextPieces[i] = g.nextPieces[i+1]
		g.nextSpecial[i] = g.nextSpecial[i+1]
	}

	//Genera nueva pieza para el último espacio
	g.nextPieces[2] = rand.Intn(11) + 1
	g.nextSpecial[2] = rand.Float64() < ProbabiliSpecialPiece

	//Verifica Game Over
	if !g.canMove(0, 0) {
		g.gameOver()
	}
}

// .............................................

func (g *Game) drawNextPieces(screen *ebiten.Image) {
	previewX := GridWidth*SizeDelBlock - 520
	previewY := 190

	text.Draw(screen, "SIGUIENTE:", g.gameFont, 10, 150, (color.RGBA{150, 150, 255, 255}))

	tetrominos := map[int][][]struct{ x, y int }{
		1: { // I
			{{0, 0}, {1, 0}, {2, 0}, {3, 0}},
			{{1, -1}, {1, 0}, {1, 1}, {1, 2}},
			{{0, 1}, {1, 1}, {2, 1}, {3, 1}},
			{{2, -1}, {2, 0}, {2, 1}, {2, 2}},
		},
		2: { // O - No rota
			{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
			{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
			{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
			{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
		},
		3: { // T
			{{1, 0}, {0, 1}, {1, 1}, {2, 1}},
			{{1, 0}, {1, 1}, {2, 1}, {1, 2}},
			{{0, 1}, {1, 1}, {2, 1}, {1, 2}},
			{{1, 0}, {0, 1}, {1, 1}, {1, 2}},
		},
		4: { // L
			{{0, 0}, {0, 1}, {0, 2}, {1, 2}},
			{{0, 0}, {1, 0}, {2, 0}, {0, 1}},
			{{0, 0}, {1, 0}, {1, 1}, {1, 2}},
			{{2, 0}, {0, 1}, {1, 1}, {2, 1}},
		},
		5: { // J
			{{1, 0}, {1, 1}, {0, 2}, {1, 2}},
			{{0, 0}, {0, 1}, {1, 1}, {2, 1}},
			{{0, 0}, {1, 0}, {0, 1}, {0, 2}},
			{{0, 0}, {1, 0}, {2, 0}, {2, 1}},
		},
		6: { // Z
			{{0, 0}, {1, 0}, {1, 1}, {2, 1}},
			{{2, 0}, {1, 1}, {2, 1}, {1, 2}},
			{{0, 1}, {1, 1}, {1, 2}, {2, 2}},
			{{1, 0}, {0, 1}, {1, 1}, {0, 2}},
		},
		7: { // S
			{{1, 0}, {2, 0}, {0, 1}, {1, 1}},
			{{1, 0}, {1, 1}, {2, 1}, {2, 2}},
			{{1, 1}, {2, 1}, {0, 2}, {1, 2}},
			{{0, 0}, {0, 1}, {1, 1}, {1, 2}},
		},
		8: { // U
			{{1, 0}, {0, 0}, {0, 1}, {0, 2}, {1, 2}},
			{{0, 1}, {0, 0}, {1, 0}, {2, 0}, {2, 1}},
			{{0, 0}, {1, 0}, {1, 1}, {1, 2}, {0, 2}},
			{{0, 0}, {0, 1}, {1, 1}, {2, 1}, {2, 0}},
		},
		9: { // Pieza Especial 1
			{{2, 0}, {2, 1}, {1, 1}, {1, 2}, {0, 2}},
			{{1, 0}, {1, 1}, {1, 2}, {0, 1}, {2, 1}},
			{{0, 0}, {0, 1}, {1, 1}, {2, 1}, {2, 2}},
			{{2, 0}, {2, 1}, {1, 1}, {0, 1}, {0, 2}},
		},
		10: { // | grande
			{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}},
			{{2, 0}, {2, 1}, {2, 2}, {2, 3}, {2, 4}},
			{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}},
			{{2, 0}, {2, 1}, {2, 2}, {2, 3}, {2, 4}},
		},
		11: { // Otra
			{{0, 2}, {1, 2}, {2, 2}, {0, 1}, {0, 0}},
			{{0, 0}, {1, 0}, {2, 0}, {0, 1}, {0, 2}},
			{{0, 0}, {1, 0}, {2, 0}, {2, 1}, {2, 2}},
			{{0, 2}, {1, 2}, {2, 2}, {2, 1}, {2, 0}},
		},
	}

	// Posiciones fijas para cada pieza preview
	previewPositions := []struct{ x, y int }{
		{0, 0},     // Pieza 1
		{0, 5},     // Pieza 2
		{200, 200}, // Pieza 3
	}

	for i := 0; i < 3; i++ {
		currentShape := tetrominos[g.nextPieces[i]][0] // Usa la rotación 0

		for _, block := range currentShape {
			x := previewX + previewPositions[i].x*SizeDelBlock + block.x*SizeDelBlock
			y := previewY + previewPositions[i].y*SizeDelBlock + block.y*SizeDelBlock

			// Usa un color especial si la pieza es especial
			if g.nextSpecial[i] {
				g.drawBlock(screen, x/SizeDelBlock, y/SizeDelBlock, g.nextPieces[i], true, color.RGBA{255, 215, 0, 255})
			} else {
				g.drawBlock(screen, x/SizeDelBlock, y/SizeDelBlock, g.nextPieces[i], false, color.RGBA{255, 255, 255, 255})
			}
		}
	}
}

// .............................................

func (g *Game) playSound(name string) {
	if player, exists := g.sounds[name]; exists && player != nil {
		player.Rewind()
		player.Play()
	}
}

func (g *Game) playBGM() {
	if g.bgms[g.currentBgm] != nil {
		g.bgms[g.currentBgm].Play()
	}
}

// .............................................
// .... Dibujado de todo el juego ....
// .... Renderizado ....
func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{29, 29, 41, 255})

	switch g.Estado {
	case EstadoCompany:
		g.drawCompanyLogo(screen)
	case EstadoStart:
		g.drawStartScreen(screen)
	case EstadoPlayerName:
		g.drawPlayerName(screen)
	case EstadoPlayMenu:
		g.drawPlayMenu(screen)
	case EstadoReglas:
		g.drawRules(screen)
	case EstadoHistoria:
		g.drawHistoria(screen)
	case EstadoMenu:
		g.drawMenu(screen)
	case EstadoGame:
		g.drawGame(screen)
	case EstadoPause:
		g.drawPause(screen)
	case EstadoGameOver:
		g.drawGameOver(screen)
	case EstadoHighScores:
		g.drawHighScores(screen)
	}
}

func (g *Game) drawMenu(screen *ebiten.Image) {
	titleText := "FETRIS - FECORO @ RENGO, CHILE"
	text.Draw(screen, titleText, g.retroFont,
		PantallaWidth/2-len(titleText)*6,
		PantallaHeight/7,
		color.White)

	instructions := []string{
		"Presiona ESPACIO o ENTER para comenzar",
		"Presiona H para ver puntajes altos",
		"Presiona ← o ESC para volver al inicio",
		"Presiona S para salir",
		"",
		"--------------------------- RECUERDA ---------------------------",
		"",
		"← → para mover",
		"↓ para caída rápida,",
		"X o Espacio para caída instantánea",
		"Presiona ↑ o Z para rotar la figura",
		"Presiona P para pausar durante el juego",
		"Presiona ESC para volver al menú durante el juego",
	}

	for i, inst := range instructions {
		text.Draw(screen, inst, g.retroFont,
			PantallaWidth/2-len(inst)*6,
			PantallaHeight/4+i*31,
			color.RGBA{200, 200, 200, 255})
	}

	//Hace color de fondo(3,5,22 / RGB)
	bg := ebiten.NewImage(PantallaWidth, PantallaHeight)
	bg.Fill(color.RGBA{3, 5, 22, 255})

	//Actualiza y hace draw a las partículas
	g.updateParticles()

	//Crea imagen temporal para las partículas
	particleImg := ebiten.NewImage(3, 3)
	particleImg.Fill(color.RGBA{255, 255, 255, 255})

	//Dibuja cada partícula
	for _, p := range g.particles {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(-1.5, -1.5) //Centra la partícula
		op.GeoM.Scale(p.size, p.size)
		op.GeoM.Translate(p.x, p.y)

		//Configura su color y transparencia
		op.ColorM.Scale(1, 1, 1, p.alpha*0.3) //Ajusta el 0.3 para cambiar la opacidad general

		screen.DrawImage(particleImg, op)
	}
}

func (g *Game) drawGame(screen *ebiten.Image) {
	//Vemos draw del fondo del grid
	gridBg := ebiten.NewImage(GridWidth*TamañoCell, GridHeight*TamañoCell)
	gridBg.Fill(color.RGBA{40, 40, 40, 255})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64((PantallaWidth-GridWidth*TamañoCell)/2), 10)
	screen.DrawImage(gridBg, op)

	//Cargar imagen de fondo
	if g.background2Image == nil {
		file, err := os.Open("componentes/background3.png")
		if err != nil {
			log.Fatal(err)
		}
		img, err := png.Decode(file)
		if err != nil {
			log.Fatal(err)
		}
		file.Close()
		g.background2Image = ebiten.NewImageFromImage(img)
	}

	//Dibuja la imagen de fondo en la pantalla
	screen.DrawImage(g.background2Image, nil)

	//Dibujamos marco alrededor del grid
	frameColor := color.RGBA{100, 100, 100, 255}
	for x := 0; x < GridWidth; x++ {
		g.drawBlock(screen, x, -1, 0, false, frameColor)
		g.drawBlock(screen, x, GridHeight, 0, false, frameColor)
	}
	for y := -1; y <= GridHeight; y++ {
		g.drawBlock(screen, -1, y, 0, false, frameColor)
		g.drawBlock(screen, GridWidth, y, 0, false, frameColor)
	}

	//Dibujar el grid
	for y := 0; y < GridHeight; y++ {
		for x := 0; x < GridWidth; x++ {
			if g.grid[y][x] != 0 {
				g.drawBlock(screen, x, y, g.grid[y][x], false, color.RGBA{255, 255, 255, 255})
			}
		}
	}

	//Dibuja pieza cayendo
	if g.Estado == EstadoGame {
		//Se definen las formas de los tetrominos con sus 4 rotaciones
		tetrominos := map[int][][]struct{ x, y int }{
			1: { // I
				{{0, 0}, {1, 0}, {2, 0}, {3, 0}},
				{{1, -1}, {1, 0}, {1, 1}, {1, 2}},
				{{0, 1}, {1, 1}, {2, 1}, {3, 1}},
				{{2, -1}, {2, 0}, {2, 1}, {2, 2}},
			},
			2: { // O - No rota
				{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
				{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
				{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
				{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
			},
			3: { // T
				{{1, 0}, {0, 1}, {1, 1}, {2, 1}},
				{{1, 0}, {1, 1}, {2, 1}, {1, 2}},
				{{0, 1}, {1, 1}, {2, 1}, {1, 2}},
				{{1, 0}, {0, 1}, {1, 1}, {1, 2}},
			},
			4: { // L
				{{0, 0}, {0, 1}, {0, 2}, {1, 2}},
				{{0, 0}, {1, 0}, {2, 0}, {0, 1}},
				{{0, 0}, {1, 0}, {1, 1}, {1, 2}},
				{{2, 0}, {0, 1}, {1, 1}, {2, 1}},
			},
			5: { // J
				{{1, 0}, {1, 1}, {0, 2}, {1, 2}},
				{{0, 0}, {0, 1}, {1, 1}, {2, 1}},
				{{0, 0}, {1, 0}, {0, 1}, {0, 2}},
				{{0, 0}, {1, 0}, {2, 0}, {2, 1}},
			},
			6: { // Z
				{{0, 0}, {1, 0}, {1, 1}, {2, 1}},
				{{2, 0}, {1, 1}, {2, 1}, {1, 2}},
				{{0, 1}, {1, 1}, {1, 2}, {2, 2}},
				{{1, 0}, {0, 1}, {1, 1}, {0, 2}},
			},
			7: { // S
				{{1, 0}, {2, 0}, {0, 1}, {1, 1}},
				{{1, 0}, {1, 1}, {2, 1}, {2, 2}},
				{{1, 1}, {2, 1}, {0, 2}, {1, 2}},
				{{0, 0}, {0, 1}, {1, 1}, {1, 2}},
			},
			8: { // U
				{{1, 0}, {0, 0}, {0, 1}, {0, 2}, {1, 2}},
				{{0, 1}, {0, 0}, {1, 0}, {2, 0}, {2, 1}},
				{{0, 0}, {1, 0}, {1, 1}, {1, 2}, {0, 2}},
				{{0, 0}, {0, 1}, {1, 1}, {2, 1}, {2, 0}},
			},
			9: { // Pieza Especial 1
				{{2, 0}, {2, 1}, {1, 1}, {1, 2}, {0, 2}},
				{{1, 0}, {1, 1}, {1, 2}, {0, 1}, {2, 1}},
				{{0, 0}, {0, 1}, {1, 1}, {2, 1}, {2, 2}},
				{{2, 0}, {2, 1}, {1, 1}, {0, 1}, {0, 2}},
			},
			10: { // | grande
				{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}},
				{{2, 0}, {2, 1}, {2, 2}, {2, 3}, {2, 4}},
				{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}},
				{{2, 0}, {2, 1}, {2, 2}, {2, 3}, {2, 4}},
			},
			11: { // Otra
				{{0, 2}, {1, 2}, {2, 2}, {0, 1}, {0, 0}},
				{{0, 0}, {1, 0}, {2, 0}, {0, 1}, {0, 2}},
				{{0, 0}, {1, 0}, {2, 0}, {2, 1}, {2, 2}},
				{{0, 2}, {1, 2}, {2, 2}, {2, 1}, {2, 0}},
			},
		}

		//Dibuja preview de las próximas piezas
		g.drawNextPieces(screen)

		//Rotar la pieza cuando se presiona Z o Arriba
		if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyUp) {
			//Incrementa la rotación y hace que vuelva a 0 después de 4
			g.fallingRotation = (g.fallingRotation + 1) % 4

			//Verificamos si la nueva rotación es válida
			if !g.isValidPosition(tetrominos[g.fallingCol][g.fallingRotation]) {
				//Si no es válida, intentar rotar en sentido horario
				g.fallingRotation = (g.fallingRotation + 3) % 4

				//Si aún no es válida, volver a la rotación anterior
				if !g.isValidPosition(tetrominos[g.fallingCol][g.fallingRotation]) {
					g.fallingRotation = (g.fallingRotation + 1) % 4
				}
			}
		}

		//Obtenemos la forma del tetromino actual con su rotación
		if blocks := tetrominos[g.fallingCol][g.fallingRotation]; len(blocks) > 0 {
			for _, block := range blocks {
				x := g.fallingX + block.x
				y := g.fallingY + block.y
				g.drawBlock(screen, x, y, g.fallingCol, g.fallingSpecial, color.RGBA{255, 255, 255, 255})
			}
		}
	}

	//Dibujamos UI respecto a la posición de la pantalla
	uiPadding := 90
	uiTextHeight := 50
	uiX := PantallaWidth - 100 - uiPadding
	uiY := uiPadding - 40

	text.Draw(screen, fmt.Sprintf("Nivel: %d", g.level), g.gameFont,
		uiX, uiY, color.RGBA{225, 225, 225, 255})
	uiY += uiTextHeight

	text.Draw(screen, fmt.Sprintf("Puntos: %d", g.score), g.gameFont,
		uiX, uiY, color.RGBA{225, 225, 225, 255})
	uiY += uiTextHeight

	text.Draw(screen, fmt.Sprintf("Tiempo: %02d", g.timer), g.gameFont,
		uiX, uiY, color.RGBA{225, 225, 225, 255})

	if g.message != "" {
		text.Draw(screen, g.message, g.retroFont,
			PantallaWidth/2-len(g.message)*4,
			PantallaHeight-60,
			color.RGBA{255, 220, 100, 255})
	}

	//Dibujo del nombre del jugador
	playerText := fmt.Sprintf("PLAYER: ")
	text.Draw(screen, playerText, g.gameFont,
		10, // posición X
		50, // posición Y
		(color.RGBA{255, 120, 120, 255}))

	playerText2 := fmt.Sprintf("%s", g.playerName)
	text.Draw(screen, playerText2, g.gameFont,
		10,  // posición X
		100, // posición Y
		(color.RGBA{255, 120, 120, 255}))
}

func (g *Game) isValidPosition(blocks []struct{ x, y int }) bool {
	for _, block := range blocks {
		x := g.fallingX + block.x
		y := g.fallingY + block.y

		//Verifica límites del grid
		if x < 0 || x >= GridWidth || y >= GridHeight {
			return false
		}

		//Verifica colisión con otras piezas
		if y >= 0 && g.grid[y][x] != 0 {
			return false
		}

	}

	return true

}

func (g *Game) drawBlock(screen *ebiten.Image, x, y int, colorIdx int, special bool, color color.RGBA) {
	op := &ebiten.DrawImageOptions{}

	//Posiciona bloque
	op.GeoM.Translate(
		float64((PantallaWidth-GridWidth*TamañoCell)/2+x*TamañoCell),
		float64(50+y*TamañoCell))

	switch colorIdx {
	case 1: // I - Cian Metálico
		op.ColorM.Scale(0.8, 1, 1, 1)
		op.ColorM.Translate(0.1, 0.1, 0.1, 0)

	case 2: // O - Amarillo Neón
		op.ColorM.Scale(1, 1, 0.7, 1)
		op.ColorM.Translate(0.1, 0.1, 0.1, 0)

	case 3: // T - Violeta Fosforescente
		op.ColorM.Scale(0.8, 0.5, 0.8, 1)
		op.ColorM.Translate(0.1, 0.1, 0.1, 0)

	case 4: // L - Naranja Brillante
		op.ColorM.Scale(1, 0.8, 0.5, 1)
		op.ColorM.Translate(0.1, 0.1, 0.1, 0)

	case 5: // J - Azul Eléctrico
		op.ColorM.Scale(0.5, 0.5, 1, 1)
		op.ColorM.Translate(0.1, 0.1, 0.1, 0)

	case 6: // Z - Rojo Rubí
		op.ColorM.Scale(1, 0.5, 0.5, 1)
		op.ColorM.Translate(0.1, 0.1, 0.1, 0)

	case 7: // S - Verde Esmeralda
		op.ColorM.Scale(0.5, 1, 0.5, 1)
		op.ColorM.Translate(0.1, 0.1, 0.1, 0)

	case 8: // U - Negro

	case 9: // Pieza Especial - Multicolor/Arcoiris
		t := float64(time.Now().UnixNano()/int64(time.Millisecond)) / 1000.0
		r := math.Sin(t)*0.5 + 0.5
		g := math.Sin(t+2.0*math.Pi/3.0)*0.5 + 0.5
		b := math.Sin(t+4.0*math.Pi/3.0)*0.5 + 0.5
		op.ColorM.Scale(r, g, b, 1)
		op.ColorM.Translate(0.1, 0.1, 0.1, 0)

	case 10: // | grande - Azul Claro
		op.ColorM.Scale(0.7, 0.7, 1, 1)
		op.ColorM.Translate(0.1, 0.1, 0.1, 0)
	case 11: // Otra - Rojo Vermellón
		op.ColorM.Scale(0.7, 0.2, 0.8, 1)
		op.ColorM.Translate(0.1, 0.1, 0.1, 0)

	default:
		op.ColorM.Scale(float64(color.R)/255, float64(color.G)/255, float64(color.B)/255, float64(color.A)/255)
		op.ColorM.Translate(0.1, 0.1, 0.1, 0)
	}

	screen.DrawImage(g.blockImage, op)

	if special {
		specialOp := &ebiten.DrawImageOptions{}
		specialOp.GeoM = op.GeoM
		screen.DrawImage(g.specialMarks["star"], specialOp)
	}
}

func (g *Game) drawPause(screen *ebiten.Image) {
	g.drawGame(screen)

	overlay := ebiten.NewImage(PantallaWidth, PantallaHeight)
	overlay.Fill(color.RGBA{0, 0, 0, 128})
	screen.DrawImage(overlay, nil)

	pauseText := "PAUSA"
	text.Draw(screen, pauseText, g.retroFont,
		PantallaWidth/2-len(pauseText)*8,
		PantallaHeight/2,
		color.White)
}

func (g *Game) drawGameOver(screen *ebiten.Image) {
	g.drawGame(screen)

	overlay := ebiten.NewImage(PantallaWidth, PantallaHeight)
	overlay.Fill(color.RGBA{0, 0, 0, 180})
	screen.DrawImage(overlay, nil)

	gameOverText := "GAME OVER"
	text.Draw(screen, gameOverText, g.retroFont,
		PantallaWidth/2-len(gameOverText)*6,
		PantallaHeight/2-40,
		color.RGBA{255, 50, 50, 255})

	scoreText := fmt.Sprintf("Puntaje Final: %d", g.score)
	text.Draw(screen, scoreText, g.retroFont,
		PantallaWidth/2-len(scoreText)*6,
		PantallaHeight/2,
		color.White)

	restartText := "Presiona ESPACIO o ESC para volver"
	text.Draw(screen, restartText, g.retroFont,
		PantallaWidth/2-len(restartText)*6,
		PantallaHeight/2+40,
		color.RGBA{200, 200, 200, 255})
}

func (g *Game) drawHighScores(screen *ebiten.Image) {
	titleText := "MEJORES PUNTAJES"
	text.Draw(screen, titleText, g.retroFont,
		PantallaWidth/2-len(titleText)*6,
		40,
		color.White)

	for i, score := range g.highScores {
		scoreText := fmt.Sprintf("%d. %s - %d pts (Nivel %d)",
			i+1, score.Name, score.Score, score.Level)
		text.Draw(screen, scoreText, g.retroFont,
			PantallaWidth/2-len(scoreText)*6,
			100+i*30,
			color.RGBA{200, 200, 200, 255})
	}

	backText := "Presiona ESC o ← para volver"
	text.Draw(screen, backText, g.retroFont,
		PantallaWidth/2-len(backText)*6,
		PantallaHeight-40,
		color.RGBA{150, 150, 150, 255})
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return PantallaWidth, PantallaHeight
}

// Funciones  (las por si acaso)
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ...............................................................
// .... Función para hacer todo el setup del juego ....
func main() {
	ebiten.SetWindowSize(PantallaWidth, PantallaHeight)
	ebiten.SetWindowTitle("FETRIS")
	ebiten.SetWindowResizable(true)

	rand.Seed(time.Now().UnixNano())

	//Colocar ícono de la ventana
	iconFile, err := os.Open("componentes/icon.png")
	if err != nil {
		log.Fatal(err)
	}
	iconImg, err := png.Decode(iconFile)
	if err != nil {
		log.Fatal(err)
	}
	iconFile.Close()

	ebiten.SetWindowIcon([]image.Image{iconImg})

	//Inicializa el contexto global de audio si no está ya inicializado
	if globalAudioContext == nil {
		globalAudioContext = audio.NewContext(SampleRate)
	}

	//Dios de juego
	game := NewGame()

	//Inicializa audio
	if err := game.initAudio(); err != nil {
		log.Fatalf("Error al inicializar el audio: %v", err)
	}

	if err := ebiten.RunGame(NewGame()); err != nil {
		log.Fatal(err)
	}
}

// ...............................................................
// Copyright (c) 2024 FECORO
// ...............................................................
