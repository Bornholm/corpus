DOKKU_APP ?= corpus
DOKKU_DEPLOY_URL ?= dokku@dev.lookingfora.name

dokku-deploy:
	git push $(DOKKU_DEPLOY_URL):$(DOKKU_APP) $(shell git rev-parse HEAD):refs/heads/master --force