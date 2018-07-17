package rules

type crawler interface {
	run()
	stop()
	isStopped() bool
}
