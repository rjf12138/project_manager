#include "hello_world_sdk.h"

void hello_world_SDK(bool flag)
{
    if (flag == true) {
        print_hello();
    } else {
        print_world();
    }
}