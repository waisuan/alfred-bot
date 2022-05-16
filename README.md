# About

Your friendly neighbourhood bot to aid in your every rota needs!

# Quickstart

1. Rename `.env.example` to `.env`
2. Fill in the blanks of the required environment variables.
3. Enable socket mode for your bot/app.
4. Create a `/rota` slash command for your bot/app.
5. Minimal bot/app scopes: `incoming-webhook`, `users:read`, `commands`, `chat:write`, `chat:write.customize`
6. Run a local DynamoDB instance: `docker run -p 8000:8000 amazon/dynamodb-local`
7. Execute!

# Features

1. Create a new rota w/ name and an initial list of members.
2. Update a rota's list of members.
3. Select from rotas associated to a given channel.
4. View a rota's details.

# TODOs

1. Start a rota w/ an option to select the initial on-call person.
2. Stop a running rota.
3. Delete a rota.
4. Override rota's current on-call person by selecting a different person for a given shift.
5. Alert channel when on-call person changes.
6. An option to insert/amend the duration of an on-call shift.