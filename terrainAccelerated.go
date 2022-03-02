package main

import (
	"fmt"

	"github.com/go-gl/gl/v4.6-core/gl"
)

var (
	demoShader uint32
)

const uint32size = 4

const demoShaderSource = `#version 460
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;
layout(std430, binding = 3) buffer layoutName
{
	uint data_SSBO[];
};
void main() {
	data_SSBO[gl_GlobalInvocationID.x] *= 8;
}
` + "\x00"

func prepareShaders() (err error) {
	demoShader, err = newProgram(demoShaderSource)
	return err
}

func app() {
	gl.UseProgram(demoShader)

	numbers := []uint32{1, 2, 3, 4, 5}
	fmt.Println(numbers)

	var ssbo uint32
	gl.GenBuffers(1, &ssbo)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, ssbo)
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, len(numbers)*uint32size, gl.Ptr(numbers), gl.DYNAMIC_READ)
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 3, ssbo)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, 0)

	gl.DispatchCompute(uint32(len(numbers)), 1, 1)
	gl.GetNamedBufferSubData(ssbo, 0, len(numbers)*uint32size, gl.Ptr(numbers))
	fmt.Println(numbers)

	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, ssbo)
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, len(numbers)*uint32size, gl.Ptr(numbers), gl.DYNAMIC_READ)
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 3, ssbo)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, 0)

	gl.DispatchCompute(uint32(len(numbers)), 1, 1)
	gl.GetNamedBufferSubData(ssbo, 0, len(numbers)*uint32size, gl.Ptr(numbers))
	fmt.Println(numbers)
}
