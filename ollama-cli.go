/*
 ollama-cli

 ollama-cli es una aplicación de terminal interactiva escrita en Go para chatear
 con modelos de lenguaje locales a través del servidor Ollama.

 Ofrece gestión automática del servidor, selección de modelos, historial de chat
 (contexto), prompts de sistema y una interfaz de usuario colorida con logos
 en degradado.

 Copyright (c) 2025, dani.eus79@gmail.com
 Todos los derechos reservados.

 Uso:
 go run .
 */

package main

import (
	"bufio"         // Para leer la entrada del usuario
	"bytes"         // Para el body de la petición POST
	"encoding/json" // Para parsear JSON (API de Ollama y logos.json)
	"fmt"     // Para realizar operaciones de entrada y salida (I/O) formateadas
	"io"      // Para leer el cuerpo de las respuestas HTTP
	"log"     // Para errores fatales de arranque
	"net/http"  // Para hacer peticiones a la API de Ollama
	"os"        // Para leer archivos (logos.json), entrada/salida estándar, y salir
	"os/exec"   // Para ejecutar 'ollama serve' y 'clear'/'cls'
	"os/signal" // Para capturar Ctrl+C (SIGINT)
	"regexp"  // Para parsear la respuesta
	"runtime"   // Para detectar el SO (windows, linux, darwin) en clearScreen
	"strconv"   // Para convertir la elección del usuario (string) a int
	"strings"   // Para limpiar y comparar strings
	"syscall"   // Para capturar la señal de terminación (SIGTERM)
	"time"      // Para timeouts y sleeps

"github.com/fatih/color" // Dependencia para los colores básicos
)

// --- Definición de nuestros colores básicos ---
var (
	cSuccess = color.New(color.FgGreen, color.Bold)
	cInfo    = color.New(color.FgYellow)
	cError   = color.New(color.FgRed)
	cPrompt  = color.New(color.FgCyan, color.Bold)
	cModel   = color.New(color.FgHiWhite)
)

// --- Estructura para definir colores RGB ---
type RGB struct {
	R, G, B uint8
}

// La URL base del servidor de Ollama
const ollamaURL = "http://localhost:11434/"

// --- Estructuras para parsear el JSON de /api/tags ---

type ModelsResponse struct {
	Models []ModelDetails `json:"models"`
}

type ModelDetails struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
}

// --- Estructuras para /api/generate ---

type GenerateRequest struct {
	Model   string  `json:"model"`
	Prompt  string  `json:"prompt"`
	Stream  bool    `json:"stream"`
	System  string  `json:"system,omitempty"`
	Context []int64 `json:"context,omitempty"`
}

type GenerateResponse struct {
	Response string  `json:"response"`
	Done     bool    `json:"done"`
	Context  []int64 `json:"context,omitempty"`
}

// ---------------------------------------------------

func loadArt(filename string) (map[string][]string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el archivo de logos '%s': %w", filename, err)
	}

	var artMap map[string][]string
	if err := json.Unmarshal(data, &artMap); err != nil {
		return nil, fmt.Errorf("error al parsear el JSON de logos '%s': %w", filename, err)
	}
	return artMap, nil
}

// ---------------------------------------------------

func main() {
	clearScreen()

	artMap, err := loadArt("logos.json")
	if err != nil {
		cError.Printf("Aviso: No se pudieron cargar los logos ASCII: %v\n", err)
		cInfo.Println("Continuando sin logos...")
		artMap = make(map[string][]string)
	}

	cInfo.Print("Iniciando el servidor de Ollama...")
	cmd := exec.Command("ollama", "serve")
	err = cmd.Start()
	if err != nil {
		log.Fatalf("Error al iniciar 'ollama serve': %v. ¿Está 'ollama' en tu PATH?", err)
	}

	fmt.Printf("\r")
	cInfo.Printf("Servidor de Ollama iniciado (PID: %d). Esperando a que esté listo...\n", cmd.Process.Pid)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("")
		cError.Println("Recibida señal de interrupción. Deteniendo Ollama...")
		if err := cmd.Process.Kill(); err != nil {
			cError.Printf("Error al detener el proceso de Ollama (PID: %d): %v\n", cmd.Process.Pid, err)
		}
		os.Exit(0)
	}()

	if !waitForOllama(15 * time.Second) {
		cError.Println("Ollama no respondió a tiempo. Deteniendo el proceso...")
		cmd.Process.Kill()
		os.Exit(1)
	}

	cSuccess.Println("\r¡Ollama está listo y respondiendo!                       ")

	selectedModel, err := listAndSelectModels()
	if err != nil {
		cError.Printf("Error al seleccionar el modelo: %v\n", err)
		cmd.Process.Kill()
		os.Exit(1)
	}

	if selectedModel == "" {
		cInfo.Println("No se seleccionó ningún modelo o no hay modelos disponibles. Saliendo.")
	} else {
		clearScreen()
		showLogo(selectedModel, artMap)

		fmt.Println("")
		cInfo.Print("Modelo seleccionado: ")
		cSuccess.Println(selectedModel)

		chatLoop(selectedModel, artMap)
	}

	fmt.Println("")
	cInfo.Println("Aplicación terminada. Deteniendo el servidor de Ollama...")
	if err := cmd.Process.Kill(); err != nil {
		cError.Printf("Error al detener el proceso de Ollama (PID: %d): %v\n", cmd.Process.Pid, err)
	} else {
		cSuccess.Println("Servidor de Ollama detenido.")
	}
}

func waitForOllama(timeout time.Duration) bool {
	startTime := time.Now()
	for {
		_, err := http.Get(ollamaURL)
		if err == nil {
			return true
		}
		if time.Since(startTime) > timeout {
			fmt.Println("")
			cError.Println("Tiempo de espera agotado para Ollama.")
			return false
		}
		cInfo.Print(".")
		time.Sleep(500 * time.Millisecond)
	}
}

func listAndSelectModels() (string, error) {
	resp, err := http.Get(ollamaURL + "api/tags")
	if err != nil {
		return "", fmt.Errorf("error al contactar /api/tags: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("respuesta inesperada de /api/tags: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error al leer la respuesta de /api/tags: %w", err)
	}
	var modelsResp ModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return "", fmt.Errorf("error al parsear JSON de modelos: %w", err)
	}
	if len(modelsResp.Models) == 0 {
		fmt.Println("")
		cError.Println("-------------------------------------------------")
		cError.Println("¡Atención! No tienes modelos de Ollama descargados.")
		cInfo.Println("Puedes descargar uno abriendo OTRA terminal y ejecutando:")
		cPrompt.Println("  ollama pull llama3")
		cError.Println("-------------------------------------------------")
		return "", nil
	}
	fmt.Println("")
	cInfo.Println("Modelos de Ollama disponibles localmente:")
	for i, model := range modelsResp.Models {
		fmt.Printf("%d. ", i+1)
		cModel.Println(model.Name)
	}
	scanner := bufio.NewScanner(os.Stdin)
	for {
		cPrompt.Printf("Elige un modelo (1-%d): ", len(modelsResp.Models))
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return "", fmt.Errorf("error al leer la entrada del usuario: %w", err)
			}
			return "", fmt.Errorf("entrada del usuario cancelada")
		}
		input := strings.TrimSpace(scanner.Text())
		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > len(modelsResp.Models) {
			cError.Println("Selección inválida. Por favor, introduce un número de la lista.")
			continue
		}
		selectedModelName := modelsResp.Models[choice-1].Name
		return selectedModelName, nil
	}
}

// --- Mostrar logo con degradado RGB ANSI ---
func showLogo(modelName string, artMap map[string][]string) {
	var startRGB, endRGB RGB
	modelKey := ""

	if strings.Contains(strings.ToLower(modelName), "llama") {
		modelKey = "llama"
		startRGB = RGB{R: 170, G: 0, B: 255}
		endRGB = RGB{R: 0, G: 170, B: 255}
	} else if strings.Contains(strings.ToLower(modelName), "mistral") {
		modelKey = "mistral"
		startRGB = RGB{R: 255, G: 140, B: 0}
		endRGB = RGB{R: 0, G: 130, B: 255}
	} else if strings.Contains(strings.ToLower(modelName), "gemma") {
		modelKey = "gemma"
		startRGB = RGB{R: 74, G: 144, B: 226}
		endRGB = RGB{R: 213, G: 62, B: 79}
	} else if strings.Contains(strings.ToLower(modelName), "phi3") {
		modelKey = "phi3"
		startRGB = RGB{R: 0, G: 180, B: 180}
		endRGB = RGB{R: 200, G: 200, B: 0}
	} else if strings.Contains(strings.ToLower(modelName), "qwen") {
		modelKey = "qwen"
		startRGB = RGB{R: 255, G: 100, B: 0}
		endRGB = RGB{R: 255, G: 200, B: 0}
	} else if strings.Contains(strings.ToLower(modelName), "deepseek") {
		modelKey = "deepseek"
		startRGB = RGB{R: 0, G: 200, B: 100}
		endRGB = RGB{R: 100, G: 100, B: 255}
	} else {
		for key := range artMap {
			if strings.Contains(strings.ToLower(modelName), key) {
				modelKey = key
				break
			}
		}
		startRGB = RGB{R: 240, G: 240, B: 240}
		endRGB = RGB{R: 220, G: 220, B: 220}
	}

	if art, ok := artMap[modelKey]; ok {
		printWithGradient(art, startRGB, endRGB)
	}
}

func clearScreen() {
	var cmd *exec.Cmd
	switch runtime.GOOS {
		case "windows":
			cmd = exec.Command("cmd", "/c", "cls")
		default:
			cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func chatLoop(modelName string, artMap map[string][]string) {
	scanner := bufio.NewScanner(os.Stdin)
	var currentContext []int64

	// ¡INSTRUCCIÓN MODIFICADA!
	systemPrompt := fmt.Sprintf(
		"Eres un asistente servicial. El modelo que estás usando es %s. NO eres ChatGPT.\n"+
		"Estás hablando con un usuario en una terminal.\n"+
		"Cuando el usuario te pida crear un archivo, formatea tu respuesta usando esta etiqueta especial:\n"+
		"<file:nombre.ext>\n[CONTENIDO DEL ARCHIVO AQUÍ]\n</file>\n"+
		"SOLO usa este formato para crear archivos.",
		modelName,
	)

	cInfo.Println("System Prompt cargado. Escribe 'exit' para salir o 'clear' para resetear.")

	for {
		cPrompt.Print("\n>>> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())

		if input == "exit" || input == "quit" {
			break
		}

		if input == "clear" || input == "reset" {
			currentContext = nil
			clearScreen()
			showLogo(modelName, artMap)
			fmt.Println("")
			cInfo.Print("Modelo seleccionado: ")
			cSuccess.Println(modelName)
			cInfo.Println("System Prompt cargado. Contexto reseteado.")
			continue
		}

		if input == "" {
			continue
		}

		cModel.Print("IA: ")

		// ¡MODIFICADO! sendPrompt ahora devuelve 3 valores
		fullResponse, newContext, err := sendPrompt(modelName, input, systemPrompt, currentContext)

		if err != nil {
			cError.Printf("Error al generar respuesta: %v\n", err)
			currentContext = nil
			cError.Println("Contexto reseteado debido a un error.")
		} else {
			currentContext = newContext // Guardamos el nuevo contexto

			// ¡NUEVO! Parseamos la respuesta completa en busca de archivos
			err := parseAndSaveFiles(fullResponse)
			if err != nil {
				cError.Printf("\nError al guardar archivos: %v\n", err)
			}
		}
	}
}

func sendPrompt(modelName string, prompt string, system string, context []int64) (string, []int64, error) {
	// 1. Preparar la estructura del Request
	reqData := GenerateRequest{
		Model:   modelName,
		Prompt:  prompt,
		Stream:  true,
		System:  system,
		Context: context,
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return "", nil, fmt.Errorf("error al crear JSON: %w", err)
	}

	reqBody := bytes.NewBuffer(jsonData)
	resp, err := http.Post(ollamaURL+"api/generate", "application/json", reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("error en la petición POST: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("respuesta inesperada del servidor: %s", resp.Status)
	}

	// ¡NUEVO! Un buffer para guardar la respuesta completa
	var fullResponse strings.Builder

	streamScanner := bufio.NewScanner(resp.Body)
	var genResp GenerateResponse
	var finalContext []int64 // Para guardar el contexto final

	for streamScanner.Scan() {
		line := streamScanner.Bytes()
		if err := json.Unmarshal(line, &genResp); err != nil {
			return "", nil, fmt.Errorf("error al parsear chunk de JSON: %w", err)
		}

		// Imprimimos el streaming como siempre
		cModel.Print(genResp.Response)

		// ¡NUEVO! Guardamos este trozo en el buffer
		fullResponse.WriteString(genResp.Response)

		// Cuando 'Done' es true, ese chunk contiene el contexto final
		if genResp.Done {
			finalContext = genResp.Context
			break
		}
	}

	if err := streamScanner.Err(); err != nil {
		return "", nil, fmt.Errorf("error al leer el stream de respuesta: %w", err)
	}

	fmt.Println()
	// ¡MODIFICADO! Devolvemos la respuesta completa y el contexto
	return fullResponse.String(), finalContext, nil
}

// --- Funciones para el degradado RGB ANSI ---

func lerp(start, end uint8, ratio float64) uint8 {
	return uint8(float64(start) + ratio*(float64(end)-float64(start)))
}

func printWithGradient(lines []string, startRGB, endRGB RGB) {
	maxWidth := 0
	for _, line := range lines {
		if len([]rune(line)) > maxWidth {
			maxWidth = len([]rune(line))
		}
	}
	if maxWidth == 0 {
		return
	}

	for _, line := range lines {
		for x, char := range []rune(line) {
			ratio := float64(x) / float64(maxWidth)
			r := lerp(startRGB.R, endRGB.R, ratio)
			g := lerp(startRGB.G, endRGB.G, ratio)
			b := lerp(startRGB.B, endRGB.B, ratio)
			fmt.Printf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", r, g, b, string(char))
		}
		fmt.Println()
	}
}

// --- ¡NUEVA FUNCIÓN PARA GUARDAR ARCHIVOS! ---

// parseAndSaveFiles busca etiquetas <file:...> en la respuesta y guarda el contenido.
func parseAndSaveFiles(response string) error {
	// (?s) es un flag que hace que el "." (punto) incluya saltos de línea.
	// <file:(.+?)>: Captura el nombre del archivo (Grupo 1)
	// \n(.*?)\n: Captura el contenido del archivo (Grupo 2)
	// </file>: Marca el final
	re := regexp.MustCompile(`(?s)<file:(.+?)>\n(.*?)\n<\/file>`)

	// Buscamos *todas* las coincidencias (por si pide crear varios archivos)
	matches := re.FindAllStringSubmatch(response, -1)

	if len(matches) == 0 {
		return nil // No se encontraron archivos, no es un error.
	}

	var filesWritten []string // Para informar al usuario

	for _, match := range matches {
		if len(match) != 3 {
			continue // Coincidencia inválida
		}

		filename := strings.TrimSpace(match[1])
		code := strings.TrimSpace(match[2])

		// --- ¡IMPORTANTE! Medida de seguridad ---
		// Evitamos que el LLM escriba en rutas peligrosas (ej. ../.bashrc)
		if strings.Contains(filename, "..") || strings.HasPrefix(filename, "/") || strings.HasPrefix(filename, "\\") {
			cError.Printf("\nAVISO DE SEGURIDAD: Se ha bloqueado la escritura de '%s' (ruta inválida).\n", filename)
			continue // Saltamos este archivo
		}

		// Escribimos el archivo en la carpeta actual
		err := os.WriteFile(filename, []byte(code), 0644) // 0644 = permisos de lectura/escritura
		if err != nil {
			cError.Printf("\nError al escribir el archivo '%s': %v\n", filename, err)
			continue
		}

		filesWritten = append(filesWritten, filename)
	}

	if len(filesWritten) > 0 {
		// Informamos al usuario de que la magia ha ocurrido
		cSuccess.Printf("\n✅ Archivo(s) guardado(s): %s\n", strings.Join(filesWritten, ", "))
	}

	return nil
}
