I will need some intermediate layer/file to keep track of what skills have been selected by the user and are being managed by the tool.

For example, as it stands, the only way we know what skills are installed are by looking at the project's .opencode/skills folder. If there is a skill named "showboat" but then "showboat" is a skill present in multiple registries, we don't know where it came from and thus, the skill selection screen becomes confusing.

The current setup also makes it difficult to track skill "versions".

I would like to introduce a "agent-lock.json" file somewhat similar to package-lock.json in its purpose. This file should keep track of what skills are installed in this project (that are managed by agent-manager), what registry they came from, and if it is a git repo, what commit hash.

As a separate but somewhat related change, I do not want registries to have "names". They should just have types and then the identifying info (like NickCellino/laptop-setup or ~/Code/laptop-setup/). This identifying info is what should be displayed in parenthesis on the skill list page and it should be colored to provide some distinction/visual interest.

