// Package input reads keyboard and mouse events. It only ever counts them:
// no key sequence and no typed text is kept or sent.
package input

// Options controls what is captured besides the counters.
type Options struct {
	PerApp    bool // attribute events to the foreground application
	Heatmap   bool // record cursor positions and clicks on a grid
	CellSize  int  // heatmap cell size in pixels
	Clipboard bool // count clipboard updates (the content is never read)
	Session   bool // count session locks and unlocks

	// OnEndSession is called when the user signs out or Windows shuts down,
	// so that the last partial period can be sent.
	OnEndSession func()
}
