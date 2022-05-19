#ifndef __EXE_SHELL_H__
#define __EXE_SHELL_H__

#include "basic_head.h"

extern int exe_shell_cmd(string &result, const char *format, ...);
extern int exe_shell_cmd_to_stdin(const char *format, ...);

#endif