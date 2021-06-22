#include "head.h"
#include "rwfile.h"
#include "bit_operation.h" 
#include "build_tree.h"
#include "encode.h" 
#include "decode.h"
#include "compress.h"
#include "extract.h"
#include "compress_extract.h"

void help_info(std::string prog_name)
{
	std::cerr << "Usages: " << std::endl;
	std::cerr << prog_name << " [-c/-e]  filename" << std::endl;
	std::cerr << prog_name << " [-h]" << std::endl;
	std::cerr << "OPTIONS:" << std::endl;
	std::cerr << "-c		compress" << std::endl;
	std::cerr << "-e		extract"  << std::endl;
	std::cerr << "-h		help info" << std::endl;
}

int options(int argc,char **argv)
{
	std::string opts[3] = {"-c", "-e", "-h"};

	if (argc < 2)
		goto end;

	int i;
	for (i = 0; i < 3; ++i) {
		if (opts[i].compare(argv[1]) == 0) {
			return i;
		}
	}
	if (i == 3)
		goto end;
	return i;
end:
	return -1;
}

int main(int argc,char **argv)
{
	int res = options(argc, argv);

	switch(res){
		case 0:
			RjfCompress::compress(argv[2]);
			break;
		case 1:
			RjfCompress::extract(argv[2]);
			break;
		case 2:
			help_info(argv[0]);
			break;
		case -1:
			std::cerr << "Unknow option: try \'"<< argv[0] <<" -h\' for more infomation." << std::endl;
			break;
	}
	return 0;
}
