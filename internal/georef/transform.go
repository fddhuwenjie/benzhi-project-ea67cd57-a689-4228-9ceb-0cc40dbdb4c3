package georef

import "math"

func Apply(c [6]float64, pixelX, pixelY float64) (float64, float64) {
	return c[0]*pixelX + c[1]*pixelY + c[2], c[3]*pixelX + c[4]*pixelY + c[5]
}
func Residual(c [6]float64, pixelX, pixelY, mapX, mapY float64) float64 {
	x, y := Apply(c, pixelX, pixelY)
	return math.Hypot(x-mapX, y-mapY)
}
