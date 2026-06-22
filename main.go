package main

func main() {
	if runRestartAfterUpdateHelper() || runRestartAfterElevationHelper() {
		return
	}
	runApp()
}
