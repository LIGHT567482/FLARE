CXX ?= g++
CXXFLAGS ?= -fPIC -shared -std=c++17 -O2 -Wall
PREFIX ?= /usr/local
INCLUDES = -Iinclude

TARGET = libflare-crypto.so

$(TARGET): src/flare_crypto.cpp
	$(CXX) $(CXXFLAGS) $(INCLUDES) -o $(TARGET) src/flare_crypto.cpp

.PHONY: clean install

clean:
	rm -f $(TARGET)

install: $(TARGET)
	install -d $(PREFIX)/lib $(PREFIX)/include
	install -m 755 $(TARGET) $(PREFIX)/lib/
	install -m 644 include/flare_crypto.h $(PREFIX)/include/
