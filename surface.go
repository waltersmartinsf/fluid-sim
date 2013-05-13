package main

import (
	"fmt"
	"math"
	"os"
	"simulator"
	"vector"
	"sync"
)


var _ = fmt.Printf
var _ = os.Exit

const (
	cubeWidth = 0.7 * cm
	isoThreshold = 550.0
)

var (
	pointArray [][][]float64
	minVector, maxVector vector.Vector
	mutex sync.Mutex
)



func constructSurface(particles ParticleList, cpus int) *simulator.Mesh {
	// construct bounding box
	minVector, maxVector = findBoundingBox(particles, cpus)

	// Expand bounding box to one kernelRadius on each side
	minVector = minVector.Subtract(vector.Vector{kernelRadius, kernelRadius, kernelRadius}.Scale(2))
	maxVector = maxVector.Add(vector.Vector{kernelRadius, kernelRadius, kernelRadius}.Scale(2))

	pointArray = makePointArray(minVector, maxVector)

	// compute field values at each vertex
	particles.ForEach(computeFieldValues, cpus)

	vertices := make([]vector.Vector, 0, 10)
	faces := make([][]int64, 0, 10)
	addFace := func(v1, v2, v3 vector.Vector) {
		numVerts := int64(len(vertices))
		face := []int64{numVerts, numVerts + 1, numVerts + 2}

		l := len(faces)
		if l + 1 > cap(faces) {  // reallocate
			// Allocate double what's needed, for future growth.
			newSlice := make([][]int64, (l + 1) * 2)
			copy(newSlice, faces)
			faces = newSlice
		}
		faces = faces[0:l+1]
		faces[l] = face

		l = len(vertices)
		if l + 3 > cap(vertices) {  // reallocate
			// Allocate double what's needed, for future growth.
			newSlice := make([]vector.Vector, (l + 3) * 2)
			copy(newSlice, vertices)
			vertices = newSlice
		}

		vertices = vertices[0:l+3]
		vertices[l] = v1
		vertices[l+1] = v2
		vertices[l+2] = v3
	}

	// Do the marching cubes craziness
	nXPoints := len(pointArray)
	nYPoints := len(pointArray[0])
	nZPoints := len(pointArray[0][0])
	for x := 0; x < nXPoints - 1; x++ {
		for y := 0; y < nYPoints - 1; y++ {
			for z := 0; z < nZPoints - 1; z++ {
				cubeindex := 0
				gridVal := [8]float64{pointArray[x][y][z],
                                      pointArray[x][y+1][z],
                                      pointArray[x+1][y+1][z],
                                      pointArray[x+1][y][z],
                                      pointArray[x][y][z+1],
                                      pointArray[x][y+1][z+1],
                                      pointArray[x+1][y+1][z+1],
                                      pointArray[x+1][y][z+1]}

				makeCorner := func(ix, iy, iz int) vector.Vector {
					return minVector.Add(vector.Vector{float64(ix), float64(iy), float64(iz)}.Scale(cubeWidth))
				}
				gridP := [8]vector.Vector{
						   makeCorner(x, y, z),
						   makeCorner(x, y+1, z),
						   makeCorner(x+1, y+1, z),
						   makeCorner(x+1, y, z),
						   makeCorner(x, y, z+1),
						   makeCorner(x, y+1, z+1),
						   makeCorner(x+1, y+1, z+1),
						   makeCorner(x+1, y, z+1),
				}

				if gridVal[0] < isoThreshold { cubeindex |= 1 }
				if gridVal[1] < isoThreshold { cubeindex |= 2 }
				if gridVal[2] < isoThreshold { cubeindex |= 4 }
				if gridVal[3] < isoThreshold { cubeindex |= 8 }
				if gridVal[4] < isoThreshold { cubeindex |= 16 }
				if gridVal[5] < isoThreshold { cubeindex |= 32 }
				if gridVal[6] < isoThreshold { cubeindex |= 64 }
				if gridVal[7] < isoThreshold { cubeindex |= 128 }

				/* Cube is entirely in/out of the surface */
				if edgeTable[cubeindex] != 0 {
					vertlist := getVertices(cubeindex, gridP, gridVal)

					for i := 0; triTable[cubeindex][i] != -1; i += 3 {
						v1 := vertlist[triTable[cubeindex][i]];
						v2 := vertlist[triTable[cubeindex][i+1]];
						v3 := vertlist[triTable[cubeindex][i+2]];
						addFace(v1, v2, v3)
					}
				}
			}
		}
	}

	// Make the mesh
	surfaceMesh := simulator.CreateMesh("Surface", vertices, faces)

	return surfaceMesh
	//return makeBoundingBoxMesh(minVector, maxVector)
}

func getVertices(cubeindex int, gridP [8]vector.Vector, gridVal [8]float64) [12]vector.Vector {
	var vertlist [12]vector.Vector

	// Function to interpolate between two vertices
	vertexInterp := func(p1, p2 vector.Vector, v1, v2 float64) vector.Vector {
		  if (math.Abs(isoThreshold - v1) < 0.00001) {
			  return p1
		  }
		  if (math.Abs(isoThreshold - v2) < 0.00001) {
			  return p2
		  }
		  if (math.Abs(v1 - v2) < 0.00001) {
			  return p1
		  }

		  mu := (isoThreshold  -  v1) / (v2  -  v1)

		return vector.Vector{
		  p1.X + mu * (p2.X - p1.X),
		  p1.Y + mu * (p2.Y - p1.Y),
		  p1.Z + mu * (p2.Z - p1.Z),
		}
	}

	/* Find the vertices where the surface intersects the cube */
	if edgeTable[cubeindex] & 1 != 0 {
		vertlist[0] = vertexInterp(gridP[0],gridP[1],gridVal[0],gridVal[1]);
	}
	if edgeTable[cubeindex] & 2 != 0 {
		vertlist[1] = vertexInterp(gridP[1],gridP[2],gridVal[1],gridVal[2]);
	}
	if edgeTable[cubeindex] & 4 != 0 {
		vertlist[2] = vertexInterp(gridP[2],gridP[3],gridVal[2],gridVal[3]);
	}
	if edgeTable[cubeindex] & 8 != 0 {
		vertlist[3] = vertexInterp(gridP[3],gridP[0],gridVal[3],gridVal[0]);
	}
	if edgeTable[cubeindex] & 16 != 0 {
		vertlist[4] = vertexInterp(gridP[4],gridP[5],gridVal[4],gridVal[5]);
	}
	if edgeTable[cubeindex] & 32 != 0 {
		vertlist[5] = vertexInterp(gridP[5],gridP[6],gridVal[5],gridVal[6]);
	}
	if edgeTable[cubeindex] & 64 != 0 {
		vertlist[6] = vertexInterp(gridP[6],gridP[7],gridVal[6],gridVal[7]);
	}
	if edgeTable[cubeindex] & 128 != 0 {
		vertlist[7] = vertexInterp(gridP[7],gridP[4],gridVal[7],gridVal[4]);
	}
	if edgeTable[cubeindex] & 256 != 0 {
		vertlist[8] = vertexInterp(gridP[0],gridP[4],gridVal[0],gridVal[4]);
	}
	if edgeTable[cubeindex] & 512 != 0 {
		vertlist[9] = vertexInterp(gridP[1],gridP[5],gridVal[1],gridVal[5]);
	}
	if edgeTable[cubeindex] & 1024 != 0 {
		vertlist[10] = vertexInterp(gridP[2],gridP[6],gridVal[2],gridVal[6]);
	}
	if edgeTable[cubeindex] & 2048 != 0 {
		vertlist[11] = vertexInterp(gridP[3],gridP[7],gridVal[3],gridVal[7]);
	}

	return vertlist
}

func findBoundingBox(particles ParticleList, cpus int) (vector.Vector, vector.Vector) {
	// Keep running min/max.
	xmin, ymin, zmin := make([]float64, cpus), make([]float64, cpus), make([]float64, cpus)
	xmax, ymax, zmax := make([]float64, cpus), make([]float64, cpus), make([]float64, cpus)

	for i := 0; i < cpus; i++ {
		xmin[i], ymin[i], zmin[i] = math.Inf(1), math.Inf(1), math.Inf(1)
		xmax[i], ymax[i], zmax[i] = math.Inf(-1), math.Inf(-1), math.Inf(-1)
	}
	particles.ForEach(func (particle *Particle, cpu int) {
		if particle.position.X > xmax[cpu] {
			xmax[cpu] = particle.position.X
		}
		if particle.position.Y > ymax[cpu] {
			ymax[cpu] = particle.position.Y
		}
		if particle.position.Z > zmax[cpu] {
			zmax[cpu] = particle.position.Z
		}

		if particle.position.X < xmin[cpu] {
			xmin[cpu] = particle.position.X
		}
		if particle.position.Y < ymin[cpu] {
			ymin[cpu] = particle.position.Y
		}
		if particle.position.Z < zmin[cpu] {
			zmin[cpu] = particle.position.Z
		}
	},cpus)

	// Find the bounding box
	minVector := vector.Vector{xmin[0], ymin[0], zmin[0]}
	maxVector := vector.Vector{xmax[0], ymax[0], zmax[0]}
	for i := 1; i < cpus; i++ {
		minVector.X = math.Min(minVector.X, xmin[i])
		minVector.Y = math.Min(minVector.Y, ymin[i])
		minVector.Z = math.Min(minVector.Z, zmin[i])

		maxVector.X = math.Max(maxVector.X, xmax[i])
		maxVector.Y = math.Max(maxVector.Y, ymax[i])
		maxVector.Z = math.Max(maxVector.Z, zmax[i])
	}

	return minVector, maxVector

}

func makeBoundingBoxMesh(minVector, maxVector vector.Vector) *simulator.Mesh {
	vertices := make([]vector.Vector, 8)
	faces := make([][]int64, 6)

	vertices[0] = vector.Vector{minVector.X, minVector.Y, minVector.Z}
	vertices[1] = vector.Vector{minVector.X, minVector.Y, maxVector.Z}
	vertices[2] = vector.Vector{minVector.X, maxVector.Y, minVector.Z}
	vertices[3] = vector.Vector{minVector.X, maxVector.Y, maxVector.Z}
	vertices[4] = vector.Vector{maxVector.X, minVector.Y, minVector.Z}
	vertices[5] = vector.Vector{maxVector.X, minVector.Y, maxVector.Z}
	vertices[6] = vector.Vector{maxVector.X, maxVector.Y, minVector.Z}
	vertices[7] = vector.Vector{maxVector.X, maxVector.Y, maxVector.Z}

	faces[0] = []int64{0, 1, 3, 2};
	faces[1] = []int64{0, 1, 5, 4};
	faces[2] = []int64{0, 2, 6, 4};
	faces[3] = []int64{2, 3, 7, 6};
	faces[4] = []int64{1, 3, 7, 5};
	faces[5] = []int64{4, 5, 7, 6};

	return simulator.CreateMesh("BoundingBox", vertices, faces)
}

func makePointArray(minVector, maxVector vector.Vector) [][][]float64 {
	diffVector := maxVector.Subtract(minVector)
	nXPoints := int(diffVector.X / cubeWidth) + 1
	nYPoints := int(diffVector.Y / cubeWidth) + 1
	nZPoints := int(diffVector.Z / cubeWidth) + 1
	points := make([][][]float64, nXPoints)
	for i := 0; i < nXPoints; i++ {
		points[i] = make([][]float64, nYPoints)
		for j := 0; j < nYPoints; j++ {
			points[i][j] = make([]float64, nZPoints)
		}
	}

	return points
}

func computeFieldValues(particle *Particle, cpu int) {
	boundingPosition := particle.position.Subtract(minVector)
	lowerBoundingPosition := boundingPosition.Subtract(vector.Vector{kernelRadius, kernelRadius, kernelRadius})
	upperBoundingPosition := boundingPosition.Add(vector.Vector{kernelRadius, kernelRadius, kernelRadius})
	for x := int(math.Ceil(lowerBoundingPosition.X / cubeWidth));
		x <= int(math.Floor(upperBoundingPosition.X / cubeWidth)); x++ {
		for y := int(math.Ceil(lowerBoundingPosition.Y / cubeWidth));
			y <= int(math.Floor(upperBoundingPosition.Y / cubeWidth)); y++ {
			for z := int(math.Ceil(lowerBoundingPosition.Z / cubeWidth));
				z <= int(math.Floor(upperBoundingPosition.Z / cubeWidth)); z++ {
				dist := boundingPosition.DistanceTo(vector.Vector{float64(x) * cubeWidth,
					float64(y) * cubeWidth, float64(z) * cubeWidth})
				if dist < kernelRadius {
					mutex.Lock()
					pointArray[x][y][z] += smoothingKernel(dist)
					mutex.Unlock()
				}
			}
		}
	}
}