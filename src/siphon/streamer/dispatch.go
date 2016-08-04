package streamer

import "log"

type message struct {
	appID   string
	userID  string
	payload []byte
}

// dispatcher maintains a set of connections and organizes their messages
type dispatcher struct {
	// {app_id: [*connection, *connection], etc ...}
	notificationConnections map[string][]*connection
	logReaderConnections    map[string][]*connection
	logWriterConnections    map[string][]*connection
	// {user_id: [*connection, *connection], etc ...}
	sandboxConnections map[string][]*connection

	// Broadcast messages
	broadcastNotification chan message
	broadcastLog          chan message

	// register/unregister requests from connections
	register   chan *connection
	unregister chan *connection
}

var d = dispatcher{
	notificationConnections: make(map[string][]*connection),
	logReaderConnections:    make(map[string][]*connection),
	logWriterConnections:    make(map[string][]*connection),
	sandboxConnections:      make(map[string][]*connection),
	broadcastNotification:   make(chan message),
	broadcastLog:            make(chan message),
	register:                make(chan *connection),
	unregister:              make(chan *connection),
}

func (d *dispatcher) dispatchNotification(m message) {
	log.Printf("Broadcasting notification for app %v...", m.appID)
	sandboxNotifs := d.sandboxConnections[m.userID]
	for _, n := range sandboxNotifs {
		n.sendChannel <- m.payload
	}

	appNotifs := d.notificationConnections[m.appID]
	for _, n := range appNotifs {
		n.sendChannel <- m.payload
	}
}

func (d *dispatcher) dispatchLog(m message) {
	log.Printf("Broadcasting log for app %v...", m.appID)
	logReaders := d.logReaderConnections[m.appID]
	for _, lr := range logReaders {
		lr.sendChannel <- m.payload
	}
}

func (d *dispatcher) registerConnection(c *connection) {
	// Put the connection in the appropriate collection
	switch c.connType {
	case "notifications":
		if c.appID == "*" {
			sandboxConn := d.sandboxConnections[c.userID]
			d.sandboxConnections[c.userID] = append(sandboxConn, c)
		} else {
			nc := d.notificationConnections[c.appID]
			d.notificationConnections[c.appID] = append(nc, c)
		}

	case "log_reader":
		lrc := d.logReaderConnections[c.appID]
		d.logReaderConnections[c.appID] = append(lrc, c)
	case "log_writer":
		lwc := d.logWriterConnections[c.appID]
		d.logWriterConnections[c.appID] = append(lwc, c)
	}
	log.Printf("Connection of type %v registered", c.connType)
}

func (d *dispatcher) run() {
	log.Printf("Running dispatcher")
	for {
		select {
		case c := <-d.register:
			d.registerConnection(c)
		case n := <-d.broadcastNotification:
			d.dispatchNotification(n)
		case l := <-d.broadcastLog:
			d.dispatchLog(l)
		}
	}
}
