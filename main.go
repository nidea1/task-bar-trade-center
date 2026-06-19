package main

func main() {
	if runRestartAfterUpdateHelper() {
		return
	}
	runApp()
}
