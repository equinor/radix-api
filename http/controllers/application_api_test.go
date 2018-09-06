package controllers

import "testing"

func TestGetProjectNameFromRepo(t *testing.T) {
	expected := "my-app"
	actual, _ := getProjectNameFromRepo("https://github.com/Equinor/my-app")

	if actual != expected {
		t.Errorf("GetProjectNameFromRepo: expected %s, actual %s", expected, actual)
	}
}

func TestGetCloneURLRepo(t *testing.T) {
	expected := "git@github.com:Equinor/my-app.git"
	actual, _ := getCloneURLFromRepo("https://github.com/Equinor/my-app")

	if actual != expected {
		t.Errorf("GetCloneURLFromRepo: expected %s, actual %s", expected, actual)
	}
}
