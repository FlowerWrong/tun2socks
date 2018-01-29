cmake_minimum_required(VERSION 3.0)

project(tun2socks)

set(CMAKE_C_FLAGS "-std=c99")

include_directories(${CMAKE_SOURCE_DIR})

set(MAIN_SOURCE_FILES
        main.c
        )

add_executable(tun2socks ${MAIN_SOURCE_FILES})
target_link_libraries(tun2socks
        pthread
        )
target_link_libraries(tun2socks
        ${CMAKE_SOURCE_DIR}/libtun2socks.a
        )
