package goroutines

func Start(done chan<- error) {
	done <- nil
}
