//Programado por dani.eus79@gmail.com
package main

import (
	"bufio"         // Para leer la entrada del usuario
	"bytes"         // ¡Corregido! Necesario para http.Post
	"encoding/json" // Para parsear JSON (API de Ollama y logos.json)
"fmt"
"io"      // Para leer el cuerpo de las respuestas HTTP
"log"     // Para errores fatales de arranque
"net/http"  // Para hacer peticiones a la API de Ollama
"os"        // Para leer archivos (logos.json), entrada/salida estándar, y salir
"os/exec"   // Para ejecutar 'ollama serve' y 'clear'/'cls'
"os/signal" // Para capturar Ctrl+C (SIGINT)
"runtime"   // Para detectar el SO (windows, linux, darwin) en clearScreen
"strconv"   // Para convertir la elección del usuario (string) a int
"strings"   // Para limpiar y comparar strings
"syscall"   // Para capturar la señal de terminación (SIGTERM)
"time"      // Para timeouts y sleeps

// Dependencia externa para los colores
"github.com/fatih/color"
)

// --- Definición de nuestros colores ---
var (
	cSuccess = color.New(color.FgGreen, color.Bold) // Verde brillante para éxitos
	cInfo    = color.New(color.FgYellow)            // Amarillo para información o espera
	cError   = color.New(color.FgRed)               // Rojo para errores
	cPrompt  = color.New(color.FgCyan, color.Bold)  // Cian para los prompts al usuario
	cModel   = color.New(color.FgHiWhite)           // Blanco brillante para los nombres de los modelos
)

// La URL base del servidor de Ollama
const ollamaURL = "http://localhost:11434/"

// --- Estructuras para parsear el JSON de /api/tags ---

// ModelsResponse es la estructura principal que devuelve /api/tags
type ModelsResponse struct {
	Models []ModelDetails `json:"models"`
}

// ModelDetails contiene la información de cada modelo individual
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
	Stream bool   `json:"stream"` // ¡Queremos streaming!
}

// GenerateResponse es la estructura que RECIBIMOS (en trozos)
type GenerateResponse struct {
	Response string `json:"response"` // El trozo de texto
	Done     bool   `json:"done"`     // ¿Es este el último trozo?
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
	// 1. Limpiamos la pantalla al inicio
	clearScreen()

	// 2. Cargamos los logos desde JSON
	artMap, err := loadArt("logos.json")
	if err != nil {
		// Si falla (ej. archivo no encontrado), solo avisamos. No es fatal.
		cError.Printf("Aviso: No se pudieron cargar los logos ASCII: %v\n", err)
		cInfo.Println("Continuando sin logos...")
		artMap = make(map[string][]string) // Creamos un map vacío para que 'showLogo' no falle
	}

	// 3. Iniciamos el servidor de Ollama
	cInfo.Print("Iniciando el servidor de Ollama...")

	cmd := exec.Command("ollama", "serve")
	err = cmd.Start()
	if err != nil {
		log.Fatalf("Error al iniciar 'ollama serve': %v. ¿Está 'ollama' en tu PATH?", err)
	}

	fmt.Printf("\r") // Retorno de carro para sobrescribir la línea
	cInfo.Printf("Servidor de Ollama iniciado (PID: %d). Esperando a que esté listo...\n", cmd.Process.Pid)

	// 4. Gestión de la limpieza (Shutdown) con Ctrl+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c // Espera aquí hasta que llegue una señal
		fmt.Println("") // Salto de línea después del ^C
		cError.Println("Recibida señal de interrupción. Deteniendo Ollama...")
		if err := cmd.Process.Kill(); err != nil {
			cError.Printf("Error al detener el proceso de Ollama (PID: %d): %v\n", cmd.Process.Pid, err)
		}
		os.Exit(0)
	}()

	// 5. Esperamos a que el servidor esté listo
	if !waitForOllama(15 * time.Second) {
		cError.Println("Ollama no respondió a tiempo. Deteniendo el proceso...")
		cmd.Process.Kill()
		os.Exit(1)
	}

	// \r (retorno de carro) borra la línea de "esperando..."
	cSuccess.Println("\r¡Ollama está listo y respondiendo!                       ")

	// 6. Listar y seleccionar el modelo
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
		// Limpiamos la pantalla y mostramos el logo
		clearScreen()
		showLogo(selectedModel, artMap)

		fmt.Println("") // Espacio extra
		cInfo.Print("Modelo seleccionado: ")
		cSuccess.Println(selectedModel)

		// Inicia el bucle de chat. Esta función no volverá hasta que el usuario escriba 'exit'.
		chatLoop(selectedModel)
	}

	// 8. Limpieza al salir normalmente
	fmt.Println("")
	cInfo.Println("Aplicación terminada. Deteniendo el servidor de Ollama...")
	if err := cmd.Process.Kill(); err != nil {
		cError.Printf("Error al detener el proceso de Ollama (PID: %d): %v\n", cmd.Process.Pid, err)
	} else {
		cSuccess.Println("Servidor de Ollama detenido.")
	}
}

// waitForOllama intenta conectar con Ollama hasta que lo consigue o se acaba el tiempo.
func waitForOllama(timeout time.Duration) bool {
	startTime := time.Now()
	for {
		_, err := http.Get(ollamaURL)
		if err == nil {
			// Conexión exitosa
			return true
		}
		// Si se acabó el tiempo, fallamos
		if time.Since(startTime) > timeout {
			fmt.Println("") // Salto de línea después de los puntos
			cError.Println("Tiempo de espera agotado para Ollama.")
			return false
		}
		// Esperamos un poco antes de reintentar
		cInfo.Print(".") // Puntos amarillos
		time.Sleep(500 * time.Millisecond)
	}
}

// listAndSelectModels se conecta a /api/tags, muestra los modelos y pide al usuario que elija.
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

	// Comprobar si hay modelos
	if len(modelsResp.Models) == 0 {
		fmt.Println("")
		cError.Println("-------------------------------------------------")
		cError.Println("¡Atención! No tienes modelos de Ollama descargados.")
		cInfo.Println("Puedes descargar uno abriendo OTRA terminal y ejecutando:")
		cPrompt.Println("  ollama pull llama3")
		cError.Println("-------------------------------------------------")
		return "", nil // No es un error, pero no hay nada que seleccionar
	}

	// Mostrar modelos
	fmt.Println("")
	cInfo.Println("Modelos de Ollama disponibles localmente:")
	for i, model := range modelsResp.Models {
		fmt.Printf("%d. ", i+1)
		cModel.Println(model.Name)
	}

	// Bucle de validación de entrada
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

		// Comprobar si es un número válido Y si está en el rango correcto
		if err != nil || choice < 1 || choice > len(modelsResp.Models) {
			cError.Println("Selección inválida. Por favor, introduce un número de la lista.")
			continue // Vuelve a preguntar
		}

		// Selección válida
		selectedModelName := modelsResp.Models[choice-1].Name
		return selectedModelName, nil
	}
}

// showLogo busca un logo en el map de arte y lo imprime.
func showLogo(modelName string, artMap map[string][]string) {
	for key, artLines := range artMap {
		// Comprobamos si el nombre del modelo (ej. "llama3:8b")
		// CONTIENE la clave (ej. "llama")
		if strings.Contains(strings.ToLower(modelName), key) {

			// Unimos el array de líneas en un solo string con saltos de línea
			artString := strings.Join(artLines, "\n")

			// Imprimimos el string resultante con color
			cModel.Println(artString)
			return
		}
	}
	// Si no se encuentra arte, simplemente no se imprime nada.
}

// clearScreen limpia la consola (compatible con Windows, Mac y Linux).
func clearScreen() {
	var cmd *exec.Cmd

	switch runtime.GOOS {
		case "windows":
			cmd = exec.Command("cmd", "/c", "cls") // Comando de Windows
		default:
			// "linux" y "darwin" (macOS)
			cmd = exec.Command("clear") // Comando de Unix
	}

	cmd.Stdout = os.Stdout
	cmd.Run()
}

// chatLoop es el bucle principal de la aplicación.
// Pide al usuario un prompt y llama a sendPrompt.
func chatLoop(modelName string) {
	// Reutilizamos un scanner para leer la entrada del usuario
	scanner := bufio.NewScanner(os.Stdin)

	// Bucle infinito
	for {
		// Imprimimos el prompt para el usuario (con \n para espaciar)
		cPrompt.Print("\n>>> ")

		// Esperamos a que el usuario escriba algo y presione Enter
		if !scanner.Scan() {
			break // Si hay un error o (Ctrl+D), salimos
		}

		input := strings.TrimSpace(scanner.Text())

		// Comandos para salir
		if input == "exit" || input == "quit" {
			break // Salimos del bucle for
		}

		if input == "" {
			continue // Si no escribe nada, volvemos a preguntar
		}

		// Imprimimos el prefijo de la IA (sin salto de línea)
		cModel.Print("IA: ")

		// Llamamos a la función que hace la magia
		err := sendPrompt(modelName, input)
		if err != nil {
			cError.Printf("Error al generar respuesta: %v\n", err)
		}
	}
}

// sendPrompt envía el prompt a Ollama y gestiona la respuesta en streaming.
func sendPrompt(modelName string, prompt string) error {
	// 1. Preparar la estructura del Request
	reqData := GenerateRequest{
		Model:  modelName,
		Prompt: prompt,
		Stream: true, // Importante para la respuesta palabra a palabra
	}

	// 2. Convertir la estructura a JSON
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return fmt.Errorf("error al crear JSON: %w", err)
	}

	// 3. Crear el body de la petición
	reqBody := bytes.NewBuffer(jsonData)

	// 4. Realizar la petición POST a /api/generate
	resp, err := http.Post(ollamaURL+"api/generate", "application/json", reqBody)
	if err != nil {
		return fmt.Errorf("error en la petición POST: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("respuesta inesperada del servidor: %s", resp.Status)
	}

	// 5. PROCESAR EL STREAMING
	// La respuesta es una serie de objetos JSON, uno por línea.
	// Usaremos un bufio.Scanner para leerla línea por línea.
	streamScanner := bufio.NewScanner(resp.Body)

	var genResp GenerateResponse // Estructura para reutilizar

	for streamScanner.Scan() {
		// Leemos la línea (que es un JSON)
		line := streamScanner.Bytes() // Usamos Bytes() para eficiencia

		// Parseamos el JSON de esa línea
		if err := json.Unmarshal(line, &genResp); err != nil {
			return fmt.Errorf("error al parsear chunk de JSON: %w", err)
		}

		// Imprimimos solo el trozo de texto de la respuesta.
		// Usamos fmt.Print (y el color cModel) para que fluya todo en la misma línea.
		cModel.Print(genResp.Response)

		// Si el modelo nos dice que ha terminado (Done: true),
		// salimos del bucle de streaming.
		if genResp.Done {
			break
		}
	}

	// Comprobamos si hubo algún error durante el escaneo del stream
	if err := streamScanner.Err(); err != nil {
		return fmt.Errorf("error al leer el stream de respuesta: %w", err)
	}

	// Imprimimos un salto de línea final para que el prompt '>>>'
	// aparezca en una línea nueva.
	fmt.Println()

	return nil
}

// ¡Corregido! La llave extra al final ha sido eliminada.
