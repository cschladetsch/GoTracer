package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sync"
)

type Vec3 struct {
	X, Y, Z float64
}

func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3{v.X + other.X, v.Y + other.Y, v.Z + other.Z}
}

func (v Vec3) Sub(other Vec3) Vec3 {
	return Vec3{v.X - other.X, v.Y - other.Y, v.Z - other.Z}
}

func (v Vec3) Mul(scalar float64) Vec3 {
	return Vec3{v.X * scalar, v.Y * scalar, v.Z * scalar}
}

func (v Vec3) MulVec(other Vec3) Vec3 {
	return Vec3{v.X * other.X, v.Y * other.Y, v.Z * other.Z}
}

func (v Vec3) Dot(other Vec3) float64 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
}

func (v Vec3) Normalize() Vec3 {
	length := math.Sqrt(v.Dot(v))
	return Vec3{v.X / length, v.Y / length, v.Z / length}
}

type Ray struct {
	Origin, Direction Vec3
}

type Sphere struct {
	Center Vec3
	Radius float64
	Color  Vec3
}

type Light struct {
	Position Vec3
	Color    Vec3
}

func (s *Sphere) Intersect(ray Ray) (float64, bool) {
	oc := ray.Origin.Sub(s.Center)
	a := ray.Direction.Dot(ray.Direction)
	b := 2.0 * oc.Dot(ray.Direction)
	c := oc.Dot(oc) - s.Radius*s.Radius
	discriminant := b*b - 4*a*c
	if discriminant < 0 {
		return 0, false
	}
	t := (-b - math.Sqrt(discriminant)) / (2.0 * a)
	return t, t > 0
}

func Reflect(v, n Vec3) Vec3 {
	return v.Sub(n.Mul(2 * v.Dot(n)))
}

func RandomInUnitSphere() Vec3 {
	for {
		p := Vec3{rand.Float64()*2 - 1, rand.Float64()*2 - 1, rand.Float64()*2 - 1}
		if p.Dot(p) < 1 {
			return p
		}
	}
}

func IsOccluded(ray Ray, spheres []Sphere) bool {
	for _, sphere := range spheres {
		t, hit := sphere.Intersect(ray)
		if hit && t > 0.001 {
			return true
		}
	}
	return false
}

func CheckerboardPattern(p Vec3) Vec3 {
	if (int(math.Floor(p.X))+int(math.Floor(p.Z)))%2 == 0 {
		return Vec3{0.1, 0.1, 0.1}
	}
	return Vec3{0.9, 0.9, 0.9}
}

func TraceRay(ray Ray, spheres []Sphere, lights []Light, depth, maxDepth int) Vec3 {
	if depth >= maxDepth {
		return Vec3{0, 0, 0}
	}

	var nearestSphere *Sphere
	nearestT := math.Inf(1)

	for i := range spheres {
		t, hit := spheres[i].Intersect(ray)
		if hit && t < nearestT {
			nearestT = t
			nearestSphere = &spheres[i]
		}
	}

	if nearestSphere == nil {
		return Vec3{0.2, 0.7, 0.8} // Sky color
	}

	hitPoint := ray.Origin.Add(ray.Direction.Mul(nearestT))
	normal := hitPoint.Sub(nearestSphere.Center).Normalize()

	// Check if we're inside the sphere
	if normal.Dot(ray.Direction) > 0 {
		normal = normal.Mul(-1)
	}

	color := Vec3{0, 0, 0}
	for _, light := range lights {
		lightDir := light.Position.Sub(hitPoint).Normalize()

		// Soft shadows
		shadowIntensity := 0.0
		for i := 0; i < 16; i++ {
			jitteredLight := light.Position.Add(RandomInUnitSphere().Mul(0.1))
			jitteredLightDir := jitteredLight.Sub(hitPoint).Normalize()
			jitteredShadowRay := Ray{hitPoint.Add(normal.Mul(0.001)), jitteredLightDir}
			if !IsOccluded(jitteredShadowRay, spheres) {
				shadowIntensity += 1.0 / 16.0
			}
		}

		diffuse := math.Max(0, normal.Dot(lightDir))
		sphereColor := nearestSphere.Color
		if nearestSphere.Radius > 100 { // Assuming this is the ground sphere
			sphereColor = CheckerboardPattern(hitPoint)
		}
		color = color.Add(sphereColor.MulVec(light.Color).Mul(diffuse * shadowIntensity))
	}

	// Reflection
	reflectDir := Reflect(ray.Direction, normal)
	reflectRay := Ray{hitPoint.Add(normal.Mul(0.001)), reflectDir}
	reflectColor := TraceRay(reflectRay, spheres, lights, depth+1, maxDepth)
	color = color.Add(reflectColor.Mul(0.5))

	return color
}

func RenderPixel(x, y, width, height int, spheres []Sphere, lights []Light, maxDepth int) Vec3 {
	fov := 60.0
	aspect := float64(width) / float64(height)
	px := (2*((float64(x)+0.5)/float64(width)) - 1) * math.Tan(fov/2*math.Pi/180) * aspect
	py := (1 - 2*((float64(y)+0.5)/float64(height))) * math.Tan(fov/2*math.Pi/180)
	rayDir := Vec3{px, py, -1}.Normalize()
	return TraceRay(Ray{Vec3{0, 0, 0}, rayDir}, spheres, lights, 0, maxDepth)
}

func writeBMP(filename string, width, height int, img []Vec3) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// BMP file header (14 bytes)
	f.Write([]byte{'B', 'M'})
	fileSize := uint32(14 + 40 + 3*width*height)
	binary.Write(f, binary.LittleEndian, fileSize)
	binary.Write(f, binary.LittleEndian, uint32(0)) // Reserved
	binary.Write(f, binary.LittleEndian, uint32(54)) // Pixel data offset

	// DIB header (40 bytes)
	binary.Write(f, binary.LittleEndian, uint32(40)) // DIB header size
	binary.Write(f, binary.LittleEndian, int32(width))
	binary.Write(f, binary.LittleEndian, int32(-height)) // Negative for top-down
	binary.Write(f, binary.LittleEndian, uint16(1))  // Color planes
	binary.Write(f, binary.LittleEndian, uint16(24)) // Bits per pixel
	binary.Write(f, binary.LittleEndian, uint32(0))  // No compression
	binary.Write(f, binary.LittleEndian, uint32(0))  // Image size (can be 0 for uncompressed)
	binary.Write(f, binary.LittleEndian, int32(2835)) // Horizontal resolution (72 dpi)
	binary.Write(f, binary.LittleEndian, int32(2835)) // Vertical resolution (72 dpi)
	binary.Write(f, binary.LittleEndian, uint32(0))  // Colors in color table
	binary.Write(f, binary.LittleEndian, uint32(0))  // Important color count

	// Pixel data
	for _, pixel := range img {
		binary.Write(f, binary.LittleEndian, uint8(math.Min(255, pixel.Z*255))) // Blue
		binary.Write(f, binary.LittleEndian, uint8(math.Min(255, pixel.Y*255))) // Green
		binary.Write(f, binary.LittleEndian, uint8(math.Min(255, pixel.X*255))) // Red
	}

	return nil
}

func main() {
	maxDepth := flag.Int("bounces", 10, "Maximum number of ray bounces")
	flag.Parse()

	width, height := 800, 600
	spheres := []Sphere{
		{Vec3{0, 0, -5}, 1, Vec3{0.8, 0.3, 0.3}},    // Red sphere
		{Vec3{-2, 1, -6}, 1, Vec3{0.3, 0.8, 0.3}},   // Green sphere
		{Vec3{2, 0, -4}, 1, Vec3{0.3, 0.3, 0.8}},    // Blue sphere
		{Vec3{0, -1001, 0}, 1000, Vec3{0.9, 0.9, 0.9}}, // Ground sphere
	}

	lights := []Light{
		{Vec3{-5, 5, -5}, Vec3{0.8, 0.8, 0.8}},
		{Vec3{5, 3, -5}, Vec3{0.6, 0.6, 0.6}},
		{Vec3{0, 5, -3}, Vec3{0.5, 0.5, 0.5}},
	}

	img := make([]Vec3, width*height)
	var wg sync.WaitGroup

	for y := 0; y < height; y++ {
		wg.Add(1)
		go func(y int) {
			defer wg.Done()
			for x := 0; x < width; x++ {
				img[y*width+x] = RenderPixel(x, y, width, height, spheres, lights, *maxDepth)
			}
		}(y)
	}

	wg.Wait()

	err := writeBMP("output.bmp", width, height, img)
	if err != nil {
		fmt.Println("Error writing BMP file:", err)
		return
	}

	fmt.Println("Rendering complete. Image saved as output.bmp")
}
