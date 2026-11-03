cmake_minimum_required(VERSION 3.10)
project(flare-crypto VERSION 0.1.0 LANGUAGES C CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

find_package(OpenSSL REQUIRED COMPONENTS Crypto)

add_library(flare-crypto SHARED
    src/flare_crypto.cpp
)

target_include_directories(flare-crypto PUBLIC
    $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
    $<INSTALL_INTERFACE:include>
)
target_link_libraries(flare-crypto PRIVATE OpenSSL::Crypto)

include(CTest)
if(BUILD_TESTING)
    add_executable(flare-crypto-test tests/crypto_test.cpp)
    target_link_libraries(flare-crypto-test PRIVATE flare-crypto)
    add_test(NAME flare-crypto-test COMMAND flare-crypto-test)
endif()
