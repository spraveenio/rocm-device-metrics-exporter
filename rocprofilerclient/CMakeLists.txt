#
#
#

cmake_minimum_required(VERSION 3.21.0 FATAL_ERROR)

project(rocprofilerclient LANGUAGES CXX HIP)
set(CMAKE_INSTALL_DEFAULT_COMPONENT_NAME "rocprofilerclient")
if(CMAKE_BUILD_TYPE STREQUAL "")
    set(CMAKE_BUILD_TYPE
        "RelWithDebInfo"
        CACHE STRING "Build type" FORCE)
endif()

include(GNUInstallDirs)

# always use lib instead of lib64
set(CMAKE_INSTALL_LIBDIR "lib")
#set(CMAKE_BUILD_TYPE Debug)
find_package(PkgConfig)

if(PkgConfig_FOUND)
    set(ENV{PKG_CONFIG_SYSTEM_INCLUDE_PATH} "")
    pkg_check_modules(DW libdw)

    if(DW_FOUND
       AND DW_INCLUDE_DIRS
       AND DW_LIBRARIES)
        set(libdw_INCLUDE_DIR
            "${DW_INCLUDE_DIRS}"
            CACHE FILEPATH "libdw include directory")
        set(libdw_LIBRARY
            "${DW_LIBRARIES}"
            CACHE FILEPATH "libdw libraries")
    endif()
endif()

if(NOT libdw_INCLUDE_DIR OR NOT libdw_LIBRARY)
    find_path(
        libdw_ROOT_DIR
        NAMES include/elfutils/libdw.h
        HINTS ${libdw_ROOT}
        PATHS ${libdw_ROOT})

    mark_as_advanced(libdw_ROOT_DIR)

    find_path(
        libdw_INCLUDE_DIR
        NAMES elfutils/libdw.h
        HINTS ${libdw_ROOT}
        PATHS ${libdw_ROOT}
        PATH_SUFFIXES include)

    find_library(
        libdw_LIBRARY
        NAMES dw
        HINTS ${libdw_ROOT}
        PATHS ${libdw_ROOT}
        PATH_SUFFIXES lib lib64)
endif()

include(FindPackageHandleStandardArgs)
find_package_handle_standard_args(libdw DEFAULT_MSG libdw_LIBRARY libdw_INCLUDE_DIR)

if(libdw_FOUND AND NOT TARGET libdw::libdw)
    add_library(libdw::libdw INTERFACE IMPORTED)
    if(TARGET PkgConfig::DW AND DW_FOUND)
        target_link_libraries(libdw::libdw INTERFACE PkgConfig::DW)
    else()
        target_link_libraries(libdw::libdw INTERFACE ${libdw_LIBRARY})
        target_include_directories(libdw::libdw SYSTEM INTERFACE ${libdw_INCLUDE_DIR})
    endif()
endif()

mark_as_advanced(libdw_INCLUDE_DIR libdw_LIBRARY)

# default FAIL_REGULAR_EXPRESSION for tests
set(ROCPROFILER_DEFAULT_FAIL_REGEX
    "threw an exception|Permission denied|Could not create logging file|failed with error code"
    CACHE INTERNAL "Default FAIL_REGULAR_EXPRESSION for tests")

function(rocprofiler_get_preload_env _VAR)
    set(_PRELOAD_ENV_LIBS ${ROCPROFILER_MEMCHECK_PRELOAD_ENV_VALUE} $ENV{LD_PRELOAD})

    foreach(_TARG ${ARGN})
        if(NOT TARGET ${_TARG})
            message(
                FATAL_ERROR
                    "rocprofiler_get_preload_env: '${_TARG}' is not a valid target"
                )
        endif()
        list(APPEND _PRELOAD_ENV_LIBS $<TARGET_FILE:${_TARG}>)
    endforeach()

    if(_PRELOAD_ENV_LIBS)
        string(REPLACE ";" ":" _PRELOAD_ENV "LD_PRELOAD=${_PRELOAD_ENV_LIBS}")
    endif()

    set(${_VAR}
        "${_PRELOAD_ENV}"
        PARENT_SCOPE)
endfunction()

# function for getting the LD_LIBRARY_PATH environment variable
function(rocprofiler_get_ld_library_path_env _VAR)

    set(_LDLIB_PATH "LD_LIBRARY_PATH=")
    foreach(_TARG ${ARGN})
        if(NOT TARGET ${_TARG})
            message(
                FATAL_ERROR
                    "rocprofiler_get_ld_library_path_env: '${_TARG}' is not a valid target"
                )
        endif()
        string(APPEND _LDLIB_PATH "$<TARGET_FILE_DIR:${_TARG}>:")
    endforeach()
    # append the environments current LD_LIBRARY_PATH
    string(APPEND _LDLIB_PATH "$ENV{LD_LIBRARY_PATH}")

    set(${_VAR}
        "${_LDLIB_PATH}"
        PARENT_SCOPE)
endfunction()
## same file
if(NOT CMAKE_HIP_COMPILER)
    find_program(
        amdclangpp_EXECUTABLE
        NAMES amdclang++
        HINTS ${ROCM_PATH} ENV ROCM_PATH /opt/rocm
        PATHS ${ROCM_PATH} ENV ROCM_PATH /opt/rocm
        PATH_SUFFIXES bin llvm/bin NO_CACHE)
    mark_as_advanced(amdclangpp_EXECUTABLE)

    if(amdclangpp_EXECUTABLE)
        set(CMAKE_HIP_COMPILER "${amdclangpp_EXECUTABLE}")
    endif()
endif()

foreach(_TYPE DEBUG MINSIZEREL RELEASE RELWITHDEBINFO)
    if("${CMAKE_HIP_FLAGS_${_TYPE}}" STREQUAL "")
        set(CMAKE_HIP_FLAGS_${_TYPE} "${CMAKE_CXX_FLAGS_${_TYPE}}")
    endif()
endforeach()

find_package(rocprofiler-sdk REQUIRED)

add_library(rocpclient SHARED)
target_sources(rocpclient PRIVATE rocpclient.cpp RocpCounterSampler.h)
target_link_libraries(
    rocpclient
    PRIVATE rocprofiler-sdk::rocprofiler-sdk)

set_source_files_properties(rocpctl.cpp PROPERTIES LANGUAGE HIP)
add_executable(rocpctl)
target_sources(rocpctl PRIVATE rocpctl.cpp RocpCounterSampler.h)
target_link_libraries(rocpctl PRIVATE rocpclient
                                                        Threads::Threads)

rocprofiler_get_ld_library_path_env(LIBRARY_PATH_ENV)
rocprofiler_get_preload_env(PRELOAD_ENV rocpclient)

set(rocpctl-env "${PRELOAD_ENV}" "${LIBRARY_PATH_ENV}")
