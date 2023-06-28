package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// Structs
type Ram struct {
	Total string
	Free  string
	Use   string
}

type Cpu struct {
	Pid   string
	Name  string
	User  string
	State string
	Son   []Child
}

type Child struct {
	Pid   string
	Name  string
	User  string
	State string
}

type MapReturn struct {
	Address     string
	Size        string
	Permissions string
	Device      string
	Pathname    string
}

type KillProcess struct {
	Pid string
}

// Recibe pid a realizar map
type MapProcess struct {
	Pid string
}

type SmapProcess struct {
	Pid string
}

type MemoryStats struct {
	Rss          int
	Size         int
	InitialBlock string
	FinalBlock   string
}

func main() {

	//Crear Servidor.
	router := mux.NewRouter()
	headers := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"})
	methods := handlers.AllowedMethods([]string{"GET", "POST", "DELETE"})
	origins := handlers.AllowedOrigins([]string{"*"})

	router.HandleFunc("/ram", ramPoint).Methods("GET")
	router.HandleFunc("/cpu", cpuPoint).Methods("GET")
	router.HandleFunc("/kill", killPoint).Methods("POST")
	router.HandleFunc("/map", mapPoint).Methods("POST")
	router.HandleFunc("/smap", smapPoint).Methods("POST")
	fmt.Println("Servidor corriendo en el puerto 8081")
	http.ListenAndServe(":8081", handlers.CORS(headers, methods, origins)(router)) //Levantar servidor en el puerto 8080
}

// ----------------------
// ------ End Point -----
// ----------------------

// Ram.
func ramPoint(w http.ResponseWriter, r *http.Request) {

	ram := GetDataRam() //Obtener datos de la Ram.

	jsonBytes, err := json.Marshal(ram)
	if err != nil {
		// Manejo del error si la codificación JSON falla
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonBytes)

}

// Cpu.
func cpuPoint(w http.ResponseWriter, r *http.Request) {

	cpu := GetDataCpu() //Obtener datos del CPU.

	// Convertir la lista en formato JSON
	jsonData, err := json.Marshal(cpu)
	if err != nil {
		log.Fatal(err)
	}

	// Establecer encabezados de respuesta
	w.Header().Set("Content-Type", "application/json")

	// Escribir los datos JSON en la respuesta
	w.Write(jsonData)
}

// Killpoint.
func killPoint(w http.ResponseWriter, r *http.Request) {

	data := &KillProcess{} //Estructura donde recibimos datos
	err := json.NewDecoder(r.Body).Decode(data)
	if err != nil {
		fmt.Fprintf(w, "%v", err)
		return
	}

	out, err := exec.Command("kill", data.Pid).Output()
	if err != nil {
		log.Fatal(err)
	}
	result := string(out)
	fmt.Println(result, "\nEL proceso: "+data.Pid+" a sido detenido.")
}

// Mappoint.
func mapPoint(w http.ResponseWriter, r *http.Request) {

	data := &MapProcess{} //Estructura donde recibimos datos
	err := json.NewDecoder(r.Body).Decode(data)
	if err != nil {
		fmt.Fprintf(w, "%v", err)
		return
	}

	rutaMap := fmt.Sprintf("%s%s%s", "/proc/", data.Pid, "/maps")
	out, err := exec.Command("cat", rutaMap).Output()
	if err != nil {
		log.Fatal(err)
	}
	result := string(out)
	mapResult := GetDataMap(result)

	fmt.Println(mapResult)

	// Convertir la lista en formato JSON
	jsonData, err := json.Marshal(mapResult)
	if err != nil {
		log.Fatal(err)
	}

	// Establecer encabezados de respuesta
	w.Header().Set("Content-Type", "application/json")

	// Escribir los datos JSON en la respuesta
	w.Write(jsonData)
}

// SmapPoint.
func smapPoint(w http.ResponseWriter, r *http.Request) {

	data := &SmapProcess{} //Estructura donde recibimos datos
	err := json.NewDecoder(r.Body).Decode(data)
	if err != nil {
		fmt.Fprintf(w, "%v", err)
		return
	}

	rutaSmap := fmt.Sprintf("%s%s%s", "/proc/", data.Pid, "/smaps")
	//hacer un cat del archivo smap del proceso con permisos sudo
	out, err := exec.Command("sudo", "cat", rutaSmap).Output()
	if err != nil {
		log.Fatal(err)
	}
	smapsData := string(out)
	//fmt.Println(smapsData)
	// Parsea los objetos y obtiene la información requerida
	residentSize, virtualSize, ramUsagePercentage, initialBlock, finalBlock := parseSmapsData(smapsData)

	// Imprime los resultados
	fmt.Printf("Tamaño de la memoria residente: %d MB\n", residentSize)
	fmt.Printf("Tamaño total de la memoria virtual: %d MB\n", virtualSize)
	fmt.Printf("Porcentaje de consumo de memoria RAM: %.2f%%\n", ramUsagePercentage)
	fmt.Printf("Bloque inicial: %s\n", initialBlock)
	fmt.Printf("Bloque final: %s\n", finalBlock)
}

// ----------------------------------
// ------ Metodos - Obtener Datos -----
// ----------------------------------

func parseSmapsData(smapsData string) (residentSize, virtualSize int, ramUsagePercentage float64, initialBlock string, finalBlock string) {
	var memoryStats MemoryStats

	patron := regexp.MustCompile(`VmFlags:.*`)

	blocks := patron.Split(smapsData, -1)

	for i, block := range blocks {
		fmt.Println(i, block)
		line := strings.Split(block, "\n")

		if i == 0 {
			value := strings.Split(line[0], "-")
			memoryStats.InitialBlock = value[0]
		}

		fmt.Println(i, line)
	}

	lines := strings.Split(smapsData, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "Size:") {
			fields := strings.Fields(line)
			size, _ := strconv.Atoi(fields[1])
			memoryStats.Size += size
		}
		if strings.HasPrefix(line, "Rss:") {
			fields := strings.Fields(line)
			rss, _ := strconv.Atoi(fields[1])
			memoryStats.Rss += rss
		}
	}

	serverMemorySize := getTotalServerMemory()

	// Memoria en KB
	virtualSize = memoryStats.Size
	// Memoria en MB
	virtualSize = virtualSize / 1024

	residentSize = memoryStats.Rss
	residentSize = residentSize / 1024

	// Calcula el porcentaje de consumo de memoria RAM
	ramUsagePercentage = float64(residentSize) / float64(serverMemorySize) * 100

	initialBlock = memoryStats.InitialBlock
	finalBlock = memoryStats.FinalBlock

	return residentSize, virtualSize, ramUsagePercentage, initialBlock, finalBlock
}

func getTotalServerMemory() int {
	// Implementa aquí la lógica para obtener el tamaño total de la memoria del servidor.
	// Puede variar dependiendo del sistema operativo y la configuración.
	// En este ejemplo, se devuelve un valor ficticio de 2048 MB.
	return 2048
}

func generateMemoryVisual(residentSize, totalSize int) string {
	// Implementa aquí la lógica para generar la representación visual de la memoria.
	// Puedes utilizar cualquier técnica o librería que desees.
	// En este ejemplo, se genera un gráfico simple utilizando asteriscos (*).

	visual := strings.Repeat("*", residentSize/1024)
	visual += " / "
	visual += strings.Repeat("*", totalSize/1024)

	return visual
}

// Obtener datos map.
func GetDataMap(data string) []MapReturn {
	var mapL []MapReturn
	var mapU MapReturn
	splitResult := strings.Split(data, "\n")

	for i := 0; i < (len(splitResult) - 1); i++ {
		splitLineResult := strings.Split(splitResult[i], " ")
		mapU.Address = splitLineResult[0]
		sizeSplit := strings.Split(splitLineResult[0], "-")
		sizeMin, err := strconv.ParseInt(sizeSplit[0], 16, 64)
		if err != nil {
			fmt.Println("Error al convertir el número hexa.", err)
		}
		sizeMax, err2 := strconv.ParseInt(sizeSplit[1], 16, 64)
		if err != nil {
			fmt.Println("Error al convertir el número hexa.", err2)
		}
		size := float64(sizeMax-sizeMin) / 1024
		mapU.Size = string(strconv.FormatFloat(size, 'f', 2, 64)) + " KB"
		mapU.Permissions = splitLineResult[1]
		mapU.Device = splitLineResult[3]
		mapU.Pathname = splitLineResult[len(splitLineResult)-1]
		mapL = append(mapL, mapU)
	}

	return mapL
}

// Obtener datos memoria ram.
func GetDataRam() Ram {

	jsonRam := GetRam()                          //Valores del modulo.
	var ram Ram                                  //Struct donde se alacenara los vales.
	err := json.Unmarshal([]byte(jsonRam), &ram) //Decodificador de JSON.

	if err != nil {
		fmt.Println("Error: ", err)
	}

	return ram
}

// Obtener datos del cpu.
func GetDataCpu() []Cpu {

	jsonCpu := GetProcess()                      //Valores del modulo.
	var cpu []Cpu                                //Struct donde se alacenara los vales.
	err := json.Unmarshal([]byte(jsonCpu), &cpu) //Decodificador de JSON.

	if err != nil {
		fmt.Println("Error: ", err)
	}

	//Obtener nombre del UID.
	for x, listCpu := range cpu {

		listCpu.User = listCpu.User + "-" + GetUser(listCpu.User)
		cpu[x].User = listCpu.User

		for y, listChild := range listCpu.Son {

			listChild.User = listChild.User + "-" + GetUser(listChild.User)
			cpu[x].Son[y].User = listChild.User

		}
	}

	return cpu
}

// ----------------------------
// ------ Metodos - Modulos -----
// ----------------------------

// Obtener memoria ram del modulo.
func GetRam() string {

	cmd := exec.Command("sh", "-c", "cat /proc/mem_grupo4 ") //Ejecutar modulo.
	out, err := cmd.CombinedOutput()

	if err != nil {
		log.Println(err)
	}

	output := string(out[:])
	return output
}

// Obtener procesos del modulo.
func GetProcess() string {

	cmd := exec.Command("sh", "-c", "cat /proc/cpu_grupo4 ") //Ejecutar modulo.
	out, err := cmd.CombinedOutput()

	if err != nil {
		log.Println(err.Error())
	}

	output := string(out[:])
	return output
}

// ----------------------------
// ----- Metodos - Extras -----
// ----------------------------

// Obtener usuario
func GetUser(uid string) string {

	cmd := exec.Command("getent", "passwd", uid)
	out, err := cmd.CombinedOutput()

	if err != nil {
		log.Println(err.Error())
	}

	output := string(out[:])
	user := strings.Split(output, ":")

	return user[0]
}
