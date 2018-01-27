cmake_minimum_required(VERSION 2.8)

project(tun2socks)

set(CMAKE_CXX_STANDARD 11)

include_directories()

set(MAIN_SOURCE_FILES
        main.c
        )

add_executable(tun2socks ${MAIN_SOURCE_FILES})
