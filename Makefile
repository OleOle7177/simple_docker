GOOS = linux
IMAGES_FOLDER = ./images
CONTAINERS_FOLDER = ./containers
UBUNTU_18_04_FOLDER = $(IMAGES_FOLDER)/ubuntu_18_04
UBUNTU_18_04_IMAGE_URL = https://799928.selcdn.ru/simpledocker/ubuntu_18_04.tgz

.PHONY: all
all: simple_docker ./images/ubuntu_18_04/.timestamp containers/.timestamp

simple_docker: *.go ./images/ubuntu_18_04/.timestamp
	GOOS=$(GOOS) go build -o simple_docker main.go

./images/ubuntu_18_04/.timestamp: $(UBUNTU_18_04_FOLDER)/ubuntu_18_04.tgz
	tar -zxvf $(UBUNTU_18_04_FOLDER)/ubuntu_18_04.tgz -C $(UBUNTU_18_04_FOLDER)
	touch $(UBUNTU_18_04_FOLDER)/.timestamp

$(UBUNTU_18_04_FOLDER)/ubuntu_18_04.tgz:
	wget $(UBUNTU_18_04_IMAGE_URL) -P images/ubuntu_18_04 --show-progress

$(CONTAINERS_FOLDER)/.timestamp:
	-mkdir $(CONTAINERS_FOLDER)
	touch $(CONTAINERS_FOLDER)/.timestamp

.PHONY: clean
clean:
	rm -rf simple_docker $(IMAGES_FOLDER) $(CONTAINERS_FOLDER)
