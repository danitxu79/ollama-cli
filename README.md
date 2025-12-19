# ⚡ Ollama CLI

Programado por Daniel Serrano Armenta dani.eus79@gmai.com

Una aplicación de terminal sencilla, potente y sin dependencias (¡excepto Ollama!) para chatear con modelos de lenguaje locales. Programada en Go.

![Logo de Ollama](https://ollama.com/public/ollama.png)

## 🎯 Características Principales

* **Gestión Automática:** Inicia y detiene automáticamente el servidor `ollama serve` al ejecutar y cerrar la aplicación.
* **Selección de Modelo:** Lista todos tus modelos locales y te permite elegir con cuál chatear.
* **Memoria (Contexto):** Mantiene el historial de la conversación, permitiendo al modelo "recordar" lo que se ha dicho.
* **Personalización (System Prompt):** Utiliza un *prompt* de sistema para guiar la personalidad y el comportamiento del modelo (ej. "Eres un asistente llamado Gemma, NO ChatGPT").
* **Streaming en Tiempo Real:** Muestra las respuestas palabra por palabra, igual que en la web.
* **Interfaz Colorida:** Utiliza colores para una experiencia de usuario clara y agradable en la terminal.
* **Comandos en el Chat:**
    * `exit` o `quit`: Salir de la aplicación.
    * `clear` o `reset`: Borrar el historial de la conversación actual y empezar de cero.

## 📦 Instalación

Este proyecto está diseñado para ser ejecutado desde el código fuente usando el stack de Go.

### Requisitos Previos

* **[Go (Golang)](https://go.dev/doc/install)** (versión 1.21 o superior)
* **[Ollama](https://ollama.com/)** instalado en tu sistema.
* Al menos un modelo descargado (ej: `ollama pull llama3`)

### Pasos

1.  Clona este repositorio:
    ```bash
    # Reemplaza [TU_USUARIO]/[TU_REPO] por tu URL de GitHub
    git clone [https://github.com/](https://github.com/)[TU_USUARIO]/[TU_REPO].git
    cd [TU_REPO]
    ```

2.  Instala las dependencias de Go (principalmente `fatih/color`):
    ```bash
    go mod tidy
    ```

## 🚀 Uso

Simplemente ejecuta la aplicación desde la raíz del proyecto. El programa se encargará de iniciar `ollama serve` por ti.

```bash
go run .
```

La aplicación te guiará:

1.  Esperará a que el servidor de Ollama esté listo.
2.  Te mostrará una lista de tus modelos locales para que elijas uno.
3.  ¡Empezará el chat!

### Ejemplo de Sesión

```
$ go run .
✔ ¡Ollama está listo y respondiendo!

Modelos de Ollama disponibles localmente:
1. llama3:8b
2. gemma:2b
Elige un modelo (1-2): 2

      / \
     / _ \
     \/ \/

Modelo seleccionado: gemma:2b
System Prompt cargado. Escribe 'exit' o 'clear'.

>>> hola, me llamo Dani
IA: ¡Hola Dani! Soy Gemma, ¿en qué puedo ayudarte hoy?

>>> ¿cómo me llamo?
IA: Te llamas Dani.
```

## 🛠️ Archivos del Proyecto

* **`ollama-cli.go`**: El código fuente principal de la aplicación.
* **`go.mod` / `go.sum`**: Gestión de dependencias de Go.
* **`logos.json`**: Archivo de configuración que almacena el arte ASCII para los logos de los modelos.

## 🤝 Contribuciones

Las contribuciones son bienvenidas. Si tienes una idea para una mejora o has encontrado un bug:

1.  Haz un *Fork* del repositorio.
2.  Crea una nueva rama (`git checkout -b feature/nueva-mejora`).
3.  Haz tus cambios y haz *commit* (`git commit -m "feat: Añadir nueva mejora"`).
4.  Haz *Push* a tu rama (`git push origin feature/nueva-mejora`).
5.  Abre un *Pull Request*.


<div align="center">
  <br/>
  <p><code>>_ sudo buy-me-a-coffee --theme=dark --force</code></p>
  <a href="https://www.buymeacoffee.com/danitxu" target="_blank">
    <img src="https://cdn.buymeacoffee.com/buttons/v2/default-black.png" alt="Buy Me A Coffee" style="height: 50px !important;width: 180px !important; box-shadow: 0px 3px 2px 0px rgba(190, 190, 190, 0.5) !important;-webkit-box-shadow: 0px 3px 2px 0px rgba(190, 190, 190, 0.5) !important;">
  </a>
  <br/>
</div>
