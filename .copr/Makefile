# COPR "make_srpm" method: builds the SRPM of a spec from packaging/rpm/ with
# the release archive its Source0 names (a GitHub release made with
# tools/release.sh). COPR runs, in a chroot with network:
#   make -f .copr/Makefile srpm outdir=DIR spec=SPEC
srpm:
	dnf -y install rpm-build rpmdevtools
	spectool -g -C $(outdir) $(spec)
	rpmbuild -bs --define "_sourcedir $(outdir)" --define "_srcrpmdir $(outdir)" $(spec)
