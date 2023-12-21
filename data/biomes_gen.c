// generates biome mappings
// 
// git submodule update --init --recursive
// cd cubiomes && make libcubiomes && cd ..
// gcc biomes_gen.c cubiomes/libcubiomes.a -fwrapv -lm -o biomes_gen && ./biomes_gen > biomes/colors.go

#include <stdlib.h>
#include <stdio.h>
#include "cubiomes/util.h"
#include "cubiomes/layers.h"

int main() {
	unsigned char b[256][3];
	initBiomeColors(b);
	printf("// this file is generated with biome_gen.c\n");
	printf("package biomes\n\nimport \"image/color\"\n\n");
	printf("var BiomeColors = []color.RGBA{\n");
	for(int i = 0; i < 256; i++) {
		printf("\t{%d, %d, %d, 255},\n", b[i][0], b[i][1], b[i][2]);
	}
	printf("}\n\n");
	printf("var BiomeID = map[string]int{\n");
	for(int i = 0; i < 256; i++) {
		const char* s = biome2str(MC_1_19, i);
		if(s != NULL) {
			printf("\t\"%s\": %d,\n", s, i);
		}
	}
	printf("}\n\n");
	return 0;
}