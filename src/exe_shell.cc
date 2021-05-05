#include "exe_shell.h"

int exe_shell_cmd(string &ret, const char *format, ...)
{
	if (format == nullptr) {
		return -1;
	}

	char tmp[256] = { 0 };

	va_list aptr;
	va_start(aptr, format);
	vsnprintf(tmp, 256, format, aptr);
	va_end(aptr);
	
	
	ret = "";
	FILE* fpipe = popen(tmp, "r");
	if (!fpipe) {
	  printf("CMD:%s\rpopen:[%s]\r\n", tmp, strerror(errno));
	  return -1;
	}

	//将数据流读取到buf中
	const int N_MAX_READ_BUF = 2048;
	char result[N_MAX_READ_BUF] = {'0'};
	int read_len = 0;
	do {
		read_len = fread(result, 1, N_MAX_READ_BUF, fpipe);
		if (read_len > 0) {
		  ret.append(result, read_len);
		} else {
		  break;
		}
	} while (read_len == N_MAX_READ_BUF);
    
	pclose(fpipe); 

	return 0;
}