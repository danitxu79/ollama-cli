//Programado por dani.eus79@gmail.com
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
)

// --- Definición de nuestros colores ---
var (
	cSuccess = color.New(color.FgGreen, color.Bold)
	cInfo    = color.New(color.FgYellow)
	cError   = color.New(color.FgRed)
	cPrompt  = color.New(color.FgCyan, color.Bold)
	cModel   = color.New(color.FgHiWhite)
)

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

// GenerateRequest es la estructura que ENVIAMOS a /api/generate
type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	// ¡NUEVO CAMPO!
	System  string  `json:"system,omitempty"`
	Context []int64 `json:"context,omitempty"`
}

// GenerateResponse es la estructura que RECIBIMOS (en trozos)
type GenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	// ¡MODIFICADO! Añadimos el campo de contexto
	// 'omitempty' para que no falle si un chunk no lo trae
	Context []int64 `json:"context,omitempty"`
}

// ---------------------------------------------------

// loadArt carga el arte ASCII desde un archivo JSON
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
	// ... (1. clearScreen y 2. loadArt no cambian) ...
	clearScreen()
	artMap, err := loadArt("logos.json")
	if err != nil {
		cError.Printf("Aviso: No se pudieron cargar los logos ASCII: %v\n", err)
		cInfo.Println("Continuando sin logos...")
		artMap = make(map[string][]string)
	}

	// ... (3. Iniciar Ollama, 4. Gestión de limpieza, 5. waitForOllama, 6. listAndSelectModels no cambian) ...
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

	// 7. Lógica principal de la aplicación (post-selección)
	if selectedModel == "" {
		cInfo.Println("No se seleccionó ningún modelo o no hay modelos disponibles. Saliendo.")
	} else {
		clearScreen()
		showLogo(selectedModel, artMap)

		fmt.Println("")
		cInfo.Print("Modelo seleccionado: ")
		cSuccess.Println(selectedModel)
		cInfo.Println("Escribe 'exit' para salir o 'clear' para resetear la conversación.")


		// ¡MODIFICADO! Pasamos 'artMap' a chatLoop por si reseteamos
		chatLoop(selectedModel, artMap)
	}

	// ... (8. Limpieza al salir no cambia) ...
	fmt.Println("")
	cInfo.Println("Aplicación terminada. Deteniendo el servidor de Ollama...")
	if err := cmd.Process.Kill(); err != nil {
		cError.Printf("Error al detener el proceso de Ollama (PID: %d): %v\n", cmd.Process.Pid, err)
	} else {
		cSuccess.Println("Servidor de Ollama detenido.")
	}
}

// ... (waitForOllama, listAndSelectModels, showLogo, clearScreen no cambian) ...
// (Omitidos por brevedad, son idénticos a la versión anterior)

// waitForOllama (función idéntica)
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

// listAndSelectModels (función idéntica)
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

// showLogo (función idéntica)
func showLogo(modelName string, artMap map[string][]string) {
	for key, artLines := range artMap {
		if strings.Contains(strings.ToLower(modelName), key) {
			artString := strings.Join(artLines, "\n")
			cModel.Println(artString)
			return
		}
	}
}

// clearScreen (función idéntica)
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


// ¡MODIFICADO! chatLoop ahora define el System Prompt
func chatLoop(modelName string, artMap map[string][]string) {
	scanner := bufio.NewScanner(os.Stdin)
	var currentContext []int64

	// ¡NUEVO! Definimos la personalidad de nuestro bot.
	// Esto combate la alucinación de "ChatGPT".
	systemPrompt := "Eres un asistente servicial llamado Gemma. NO eres ChatGPT. Estás hablando con un usuario en una terminal."

	// Informamos al usuario de las nuevas instrucciones
	cInfo.Println("System Prompt cargado. Escribe 'exit' o 'clear'.")

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
			// ¡MODIFICADO! Volvemos a mostrar el aviso
			cInfo.Println("System Prompt cargado. Contexto reseteado.")
			continue
		}

		if input == "" {
			continue
		}

		cModel.Print("IA: ")

		// ¡MODIFICADO! Pasamos el systemPrompt.
		// Nota: El systemPrompt solo se usa de verdad en la *primera* petición
		// (cuando currentContext es nil), pero la API de Ollama gestiona esto.
		newContext, err := sendPrompt(modelName, input, systemPrompt, currentContext)

		if err != nil {
			cError.Printf("Error al generar respuesta: %v\n", err)
			currentContext = nil
			cError.Println("Contexto reseteado debido a un error.")
		} else {
			currentContext = newContext
		}
	}
}

// ¡MODIFICADO! sendPrompt ahora acepta el systemPrompt
func sendPrompt(modelName string, prompt string, system string, context []int64) ([]int64, error) {
	// 1. Preparar la estructura del Request
	reqData := GenerateRequest{
		Model:   modelName,
		Prompt:  prompt,
		Stream:  true,
		System:  system, // ¡NUEVO!
		Context: context,
	}

	jsonData, err := json.Marshal(reqData)
	// ... (el resto de la función sendPrompt es IDÉNTICA) ...
	if err != nil {
		return nil, fmt.Errorf("error al crear JSON: %w", err)
	}

	reqBody := bytes.NewBuffer(jsonData)
	resp, err := http.Post(ollamaURL+"api/generate", "application/json", reqBody)
	if err != nil {
		return nil, fmt.Errorf("error en la petición POST: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("respuesta inesperada del servidor: %s", resp.Status)
	}

	streamScanner := bufio.NewScanner(resp.Body)
	var genResp GenerateResponse
	var finalContext []int64

	for streamScanner.Scan() {
		line := streamScanner.Bytes()
		if err := json.Unmarshal(line, &genResp); err != nil {
			return nil, fmt.Errorf("error al parsear chunk de JSON: %w", err)
		}

		cModel.Print(genResp.Response)

		if genResp.Done {
			finalContext = genResp.Context
			break
		}
	}

	if err := streamScanner.Err(); err != nil {
		return nil, fmt.Errorf("error al leer el stream de respuesta: %w", err)
	}

	fmt.Println()
	return finalContext, nil
}
