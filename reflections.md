# Reflections / Deliberations

The following file will contain our reflections on choices made by our 
team throughout the semester

**Choice of programming language**: We have decided to use go as the language for rewriting the 
code. We were deciding between go, ruby and rust. We heard that Go was gaining popularity in the 
industry and therefore we decided to choose Go.

**Choice of framework**: We were deliberating between standard library and Gin. The advantage of using Gin is that it's the most popular framework, we could use advanced parameters in routing, wouldnt have to write manual error handling and validation. The pros of using standard library is: We can learn core go philosophy, we are not dependent on third parties, and learning it this way will help us understand many go projects. For the scope of this project, we decided to go with standard library.

**Choice of issue tracker**: We have decided to use github Projects as an issue tracker. We wanted 
to have as much of our documentation in the same place, therefore for us it was quite an obvious 
choice to go with a project for our github organization.

**Reflections of file structure**: We decided to go with a mono repo. Therefore we are making a new folder in our repository called "python_legacy" containing the legacy code. After that we are moving the db and schemas out of that folder and into a new folder for db structure. Lastly we are making a new folder for the new Go code.

**Reflections on gradual rewriting**: We've decided to make small changes to gradually implement the new endpoints, making sure they're tested and working. When they are working, we comment out the corresponding legacy code.

**Branching strategy**: Branches should be kept short lived only fixing one endpoint at a time. 
