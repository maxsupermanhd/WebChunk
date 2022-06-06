// generates biome mappings
// 
// git submodule update --init --recursive
// cd cubiomes && make libcubiomes && cd ..
// gcc biomes_gen.c cubiomes/libcubiomes.a -fwrapv -lm -o biomes_gen && ./biomes_gen > biomes/colors.go

#include <stdlib.h>
#include <stdio.h>
#include "cubiomes/util.h"

int main() {
	unsigned char b[256][3];
	initBiomeColors(b);
	printf("// this file is generated with biome_gen.c\n");
	printf("package biomes\n\nimport \"image/color\"\n\n");
	printf("var biomeColors = map[int]color.RGBA{\n");
	for(int i = 0; i < 256; i++) {
		printf("\t%d: {%d, %d, %d, 255},\n", i, b[i][0], b[i][1], b[i][2]);
	}
	printf("}\n");
	return 0;
}