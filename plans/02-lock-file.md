I will need some intermediate layer/file to keep track of what skills have been selected by the user and are being managed by the tool.

For example, as it stands, the only way we know what skills are installed are by looking at the project's .opencode/skills folder. If there is a skill named "showboat" but then "showboat" is a skill present in multiple registries, we don't know where it came from and thus, the skill selection screen becomes confusing.

The current setup also makes it difficult to track skill "versions".

I would like to introduce a "agent-lock.json" file somewhat similar to package-lock.json in its purpose. This file should keep track of what skills are installed in this project (that are managed by agent-manager), what registry they came from, and if it is a git repo, what commit hash. Each skill entry in this file will need to also keep track of the skill's true installed folder. Usually, this should be the same as the folder in the source registry unless a skill with this name already exists at that path, in which case, we should append "-{registryname}" to the skill folder name where {registryname} is a filepath friendly version of the registry name.

agent-manager should treat this file as the source of truth for what skills are installed, but it should try to keep the actual skills up-to-date. So just as before, we can add and remove skills. when adding a new skill, agent-manager should update agent-lock.json with the new skill and then add that skill to the filesystem. when removing a skill, agent-manager should delete the skill from the filesystem, then delete it from agent-lock.json.

As a separate but somewhat related change, I do not want registries to have "names". They should just have types and then the identifying info (like NickCellino/laptop-setup or ~/Code/laptop-setup/). This identifying info is what should be displayed in parenthesis on the skill list page and it should be colored to provide some distinction/visual interest.

