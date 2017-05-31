package events

// Data is a generic map from strings to any values that can be used as a means
// to wrap a chunk of dynamic data and pass them to event handlers.
// Event data should contain data specific to the event being fired that would
// allow handlers to make actionable response to. Such as an "damage_taken"
// event might have a map containing "source" (who did the damage), "target"
// (who received the damage), and then data about the damage itself.
type Data map[string]interface{}

// NewData returns an empty map[string]interface{} wrapped in the Data type,
// as an easy way to seen event emissions with empty data (where nil would mean
// no data).
func NewData() Data {
	return Data(make(map[string]interface{}))
}

// Clone will duplicate the data values and return a new data that is a deep
// copy of the original Data value.
func (d Data) Clone() Data {
	nd := make(Data)
	for k, v := range d {
		switch t := v.(type) {
		case Data:
			nd[k] = d.Clone()
		case map[string]interface{}:
			nd[k] = Data(t).Clone()
		default:
			nd[k] = v
		}
	}

	return nd
}
