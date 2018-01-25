# Basic Information

It's important to note that this project is a passion of mine and one that I've
been planning for a long time. Given that, I have high standards for this
project not just for my own desires but also as a promise to the community the
quality of project this will be and why it should be chosen in place of other
potential solutions/frameworks.

## Quality Assurance

### Testing

I do intend to have a test suite for everything where it makes sense to do so
and any contributed code is also expected to add/modify tests to suit the changes
associated to the changes. Only green PRs will be considered for merging so please
make sure that you include tests for your new features and that you verify that
no other features have broken in relation to your changes.

If the test suite becomes lacking then it will tell us nothing about the stability
and guarantees that the server software works as intended.

If you implement a piece, test it.

## Development Tools

### Github Issues

For all intents and purposes Github issues is the go to resource for reporting
bugs or new features that will be worked on. Any and all work should be associated
to an issue of some kind (branch naming will be discussed further down). If a
pull request is not associated with an issue it will be rejected. If you want
to add some wild new feature just create an issue for it and go from there -
issues don't have to be _approved_ to be valid for a pull request. There just
needs to be an issue.

### Github Projects

This repo has two projects associated with as they result in two separate end
goals. The primary project, Dragon MUD represents the MUD engine and capabilities
of running and building your own game. The second project represents web interface
efforts and potentially static site support on top of that. This project exists
currently but will most likely see use later in the projects lifespan.

Github Projects are used kanban style to represent the current status of of any
issue that currently exists under a project and collaborators will remain diligent
in making sure that tickets get added to their appropriate projects ASAP.

## Git Management

**NOTE:** While the Elixir work remains in an experimental status then **master**
will not be used for anything, just **experimental** acting as **develop**.

I'm electing to use a GitFlow method of managing repositories. This entails some
specific methods for managing branches and naming conventions. I will entail
some basics but if you're not familiar with it then I highly encourage looking
it up for in depth explanations.

There are two core "working" branches. The **master** branches becomes _the_ stable
implementation branch. Every commit to **master** coincides with a version change
whether it be major or minor. This allows the **master** branch to always be a go to
resource for working code. Each commit to this branch will be tagged with the
version.

The second "working" branch is **experimental**, this branch becomes the standard
'current' for all merged changes. Develop becomes the 'tip' branch representing
bleeding edge as far as features and work goes. All work for new features starts
from the **experimental** branch and is merged into the **experimental** branch. Never **master**.

To aid in working with these two branch there are 3 minor branche-types that
are short lived and designed to provide a mechanism for moving work between
the two working branches.

First and foremost are feature branche. Any work done for new features _will
always be done inside a feature branch_. I make no exceptions here. I will reject
pull requests that are not originating from a feature branch and targeting
**experimental**. All feature branches must always be up to date with **experimental**. Feature
branches will be named like **feature-issue#**. For example, if you're addressing
issue #1 then your branch should be **feature-#1** (the # should be present).
I would prefer no fancy names along side it but as long as the branch name
starts with this pattern that is sufficient. Once merged, if applicable, the
feature branch will be deleted.

To move code between **experimental** and **master** (and to simulate a "feature freeze")
a release branch will be created. The naming convention for these branches will
be **release-semver**, so for example releasing version 1.2 would be **release-1.2**.
Any additional work required for the release or to clean up features will be done
on this branch before being merged into **master** and tagged. At this time it will
also be merged back into **experimental**. These branches are deleted when the merged.

For those instances were our quality control pipeline breaks down or fails us
there are hotfix branches that address issues in **master** directly. These are
the only branches that will originate from and merge into **master** and also the
only branches that will increase the patch number in a version. The naming
convention for these branches is **hotfix-semver**. The version here is slightly
different than for releases as it represents a patch, so for example if we'd
just released version 1.2 and it has an issue we'd immediately create **hotfix-1.2.1**
to address this. Once completed we merge into **master** tagging 1.2.1 and merge
into **experimental** as well. These branches are deleted when merged.

I know it sounds complex and annoying as I'm stating I will be a stickler about
these things but I have faith it will enable a cleaner and more maintainable
repository for the long term life span of this project.

## Graduating to Collaborator

If you wish to graduate from "contributor" to "collaborator" then there are few
things that will be expected of you.

 * A history of contributions, not necessarily in volume or quantity but at least
   some history of contributions being merged into the project.
 * Discussion history around new features or issues. This is not a hard requirement
   but will be a significant bonus in determining if you should become a full
   blown collaborator.
 * The final 'requirement' is an interview with me, essentially just getting your
   ideas for the project as you see it's future and potential. I want to know if
   my vision is, at least in part, also your vision. As well as wanting to
   express the extent of my vision as it is in my mind.

That's pretty much it, really. I just want to ensure that anyone who becomes a
collaborator does so due to time and effort given to the project and not just
based on personal bias (I won't just make friends collaborators, in other words)
or prestige.

# A Huge Thank You

This section should probably be listed first, but honestly I feel if you've
at least looked at this document long enough or far enough to see this section
down here then we're good.

Essentially, I just want to say thank you so much. Any work that you've done
to improve the project, even it's just spelling errors (I am very prone to
making them) is work that I don't have to worry about any more. I am very
appreciative that you saw this project as worthy enough to devote your time,
no matter how little. And to honor that, anyone who has worked merged
into this project will be recognized in the 'Contributors' section on the
home page (in no particular order).

Again, thank you, I really do appreciate any work you can spare in realizing
my dream!
