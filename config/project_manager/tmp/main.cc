#include <iostream>
#include "hello_world_sdk.h"

int main(int argc, char **argv)
{
    if (argc == 2) {
        std::cout << argv[1] << std::endl;
        return 0;
    }
    hello_world_SDK(true);

    return 0;
}