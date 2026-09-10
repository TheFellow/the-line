package cli

import (
	"errors"
	"image/gif"
	"io"
	"math"

	"github.com/TheFellow/the-line/pkg/render"
)

// animationFrames bounds allocation before converting the frame count to int.
// Open sequences end at the last visible finish; closed laps retain elapsed
// time across the seam so cars with different lap periods wrap independently.
func animationFrames(a arguments, total float64, closed bool) (int, error) {
	duration := a.duration
	if closed {
		if duration == 0 {
			duration = total
		}
	} else {
		if duration == 0 {
			duration = total - a.at
		}
		if duration <= 0 {
			return 0, errors.New("animation starts past the end of the sequence")
		}
		if a.at >= total {
			return 0, errors.New("animation start time is outside the sequence")
		}
		duration = math.Min(duration, total-a.at)
	}
	frames := math.Ceil(duration * a.fps)
	if !finite(frames) || frames > 1800 || frames*float64(a.width)*float64(a.height) > 600e6 {
		return 0, errors.New("animation exceeds memory budget; reduce duration, fps, or image size")
	}
	return int(frames), nil
}

func writeAnimation(a arguments, r *render.Renderer, total float64, closed bool) error {
	frames, err := animationFrames(a, total, closed)
	if err != nil {
		return err
	}
	animation := gif.GIF{LoopCount: 0}
	quantizer := newGIFQuantizer(r.Frame(a.at))
	for i := 0; i < frames; i++ {
		// GIF timing is in hundredths. Distribute rounding to preserve the clock.
		begin := int(math.Round(float64(i) * 100 / a.fps))
		end := int(math.Round(float64(i+1) * 100 / a.fps))
		frame := r.Frame(a.at + float64(begin)/100)
		p := quantizer.frame(frame)
		animation.Image = append(animation.Image, p)
		animation.Delay = append(animation.Delay, max(1, end-begin))
	}
	return output(a.out, func(w io.Writer) error { return gif.EncodeAll(w, &animation) })
}
