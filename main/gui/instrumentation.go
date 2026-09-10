package main

func (g *game) instrumentationAction(key string) bool {
	switch key {
	case "line-color":
		g.renderer.CycleLineColor()
		return true
	case "chart-channel":
		g.renderer.CycleChartChannel()
		return true
	}
	return false
}
